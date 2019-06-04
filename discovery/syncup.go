package discovery

import (
	"context"
	"fmt"
        "net"
        "net/url"
        "strings"
        "time"

	"github.com/ouqiang/discovery/conf"
	"github.com/ouqiang/discovery/errors"
	"github.com/ouqiang/discovery/model"
	"github.com/ouqiang/discovery/registry"

        log "github.com/sirupsen/logrus"
)

var (
	_fetchAllURL = "http://%s/discovery/fetch/all"
)

// syncUp populates the registry information from a peer eureka node.
func (d *Discovery) syncUp() {
	nodes := d.nodes.Load().(*registry.Nodes)
	for _, node := range nodes.AllNodes() {
		if nodes.Myself(node.Addr) {
			continue
		}
		uri := fmt.Sprintf(_fetchAllURL, node.Addr)
		var res struct {
			Code int                          `json:"code"`
			Data map[string][]*model.Instance `json:"data"`
		}
		if err := d.client.Get(context.TODO(), uri, "", nil, &res); err != nil {
			log.Errorf("d.client.Get(%v) error(%v)", uri, err)
			continue
		}
		if res.Code != 0 {
			log.Errorf("service syncup from(%s) failed ", uri)
			continue
		}
		for _, is := range res.Data {
			for _, i := range is {
				_ = d.registry.Register(i, i.LatestTimestamp)
			}
		}
		// NOTE: no return, make sure that all instances from other nodes register into self.
	}
	nodes.UP()
}

func (d *Discovery) internalIP() string {
        inters, err := net.Interfaces()
        if err != nil {
                return ""
        }
        for _, inter := range inters {
                if !strings.HasPrefix(inter.Name, "lo") {
                        addrs, err := inter.Addrs()
                        if err != nil {
                                continue
                        }
                        for _, addr := range addrs {
                                if ipnet, ok := addr.(*net.IPNet); ok && !ipnet.IP.IsLoopback() {
                                        if ipnet.IP.To4() != nil {
                                                return ipnet.IP.String()
                                        }
                                }
                        }
                }
        }
        return ""
}

func (d *Discovery) regSelf() context.CancelFunc {
        _, port, _ := net.SplitHostPort(d.c.HTTPServer.Addr)
        addr := net.JoinHostPort(d.internalIP(), port)
	ctx, cancel := context.WithCancel(context.Background())
	now := time.Now().UnixNano()
	ins := &model.Instance{
		Region:   d.c.Env.Region,
		Zone:     d.c.Env.Zone,
		Env:      d.c.Env.DeployEnv,
		Hostname: d.c.Env.Host,
		AppID:    model.AppID,
		Addrs: []string{
			"http://" + addr,
		},
		Status:          model.InstanceStatusUP,
		RegTimestamp:    now,
		UpTimestamp:     now,
		LatestTimestamp: now,
		RenewTimestamp:  now,
		DirtyTimestamp:  now,
	}
	d.Register(ctx, ins, now, false)
	go func() {
		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				arg := &model.ArgRenew{
					AppID:    ins.AppID,
					Zone:     d.c.Env.Zone,
					Env:      d.c.Env.DeployEnv,
					Hostname: d.c.Env.Host,
				}
				if _, err := d.Renew(ctx, arg); err != nil && err == errors.NothingFound {
					d.Register(ctx, ins, now, false)
				}
			case <-ctx.Done():
				arg := &model.ArgCancel{
					AppID:    model.AppID,
					Zone:     d.c.Env.Zone,
					Env:      d.c.Env.DeployEnv,
					Hostname: d.c.Env.Host,
				}
				if err := d.Cancel(context.Background(), arg); err != nil {
					log.Errorf("d.Cancel(%+v) error(%v)", arg, err)
				}
				return
			}
		}
	}()
	return cancel
}

func (d *Discovery) nodesproc() {
	var (
		lastTs int64
	)
	for {
		arg := &model.ArgPolls{
			AppID:           []string{model.AppID},
			Env:             d.c.Env.DeployEnv,
			Hostname:        d.c.Env.Host,
			LatestTimestamp: []int64{lastTs},
		}
		ch, _, err := d.registry.Polls(arg)
		if err != nil && err != errors.NotModified {
			log.Errorf("d.registry(%v) error(%v)", arg, err)
			time.Sleep(time.Second)
			continue
		}
		apps := <-ch
		ins, ok := apps[model.AppID]
		if !ok || ins == nil {
			return
		}
		var (
			nodes []string
			zones = make(map[string][]string)
		)
		for _, ins := range ins.Instances {
			for _, in := range ins {
				for _, addr := range in.Addrs {
					u, err := url.Parse(addr)
					if err == nil && u.Scheme == "http" {
						if in.Zone == d.c.Env.Zone {
							nodes = append(nodes, u.Host)
						} else {
							zones[in.Zone] = append(zones[in.Zone], u.Host)
						}
					}
				}
			}
		}
		lastTs = ins.LatestTimestamp
		c := new(conf.Config)
		*c = *d.c
		c.Nodes = nodes
		c.Zones = zones
		ns := registry.NewNodes(c)
		ns.UP()
		d.nodes.Store(ns)
		log.Infof("discovery changed nodes:%v zones:%v", nodes, zones)
	}
}
