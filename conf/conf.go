package conf

import (
	"flag"
        "github.com/sirupsen/logrus"
        "io/ioutil"
	"os"
        "path/filepath"

	"github.com/ouqiang/goutil"
        "github.com/ouqiang/discovery/lib/http"
	"github.com/BurntSushi/toml"
)

var (
	confPath      string
	schedulerPath string
	region        string
	zone          string
	deployEnv     string
	hostname      string
	// Conf conf
	Conf = &Config{}
)

func init() {
        if logLevel := os.Getenv("DISCOVERY_LOG_LEVEL"); logLevel != "" {
                level, err := logrus.ParseLevel(logLevel)
                if err != nil {
                        panic(err)
                }
                logrus.SetLevel(level)
        }

	var err error
	if hostname, err = os.Hostname(); err != nil || hostname == "" {
		hostname = os.Getenv("HOSTNAME")
	}
	flag.StringVar(&confPath, "conf", "configs/app.toml", "config path")
	flag.StringVar(&region, "region", os.Getenv("REGION"), "avaliable region. or use REGION env variable, value: sh etc.")
	flag.StringVar(&zone, "zone", os.Getenv("ZONE"), "avaliable zone. or use ZONE env variable, value: sh001/sh002 etc.")
	flag.StringVar(&deployEnv, "deploy.env", os.Getenv("DEPLOY_ENV"), "deploy env. or use DEPLOY_ENV env variable, value: dev/fat1/uat/pre/prod etc.")
	flag.StringVar(&hostname, "hostname", hostname, "machine hostname")
	flag.StringVar(&schedulerPath, "scheduler", "scheduler.json", "scheduler info")
	if !filepath.IsAbs(confPath) {
	    workDir, err := goutil.WorkDir()
                if err != nil {
                        panic(err)
                }
	    confPath = filepath.Join(workDir, confPath)
        }
}

// Config config.
type Config struct {
	Nodes      []string
	Zones      map[string][]string
	HTTPServer *ServerConfig
	HTTPClient *http.ClientConfig
	Env        *Env
	Scheduler  []byte
}

// Fix fix env config.
func (c *Config) Fix() (err error) {
	if c.Env == nil {
		c.Env = new(Env)
	}
	if c.Env.Region == "" {
		c.Env.Region = region
	}
	if c.Env.Zone == "" {
		c.Env.Zone = zone
	}
	if c.Env.Host == "" {
		c.Env.Host = hostname
	}
	if c.Env.DeployEnv == "" {
		c.Env.DeployEnv = deployEnv
	}
	return
}

// Env is disocvery env.
type Env struct {
	Region    string
	Zone      string
	Host      string
	DeployEnv string
}

// ServerConfig Http Servers conf.
type ServerConfig struct {
	Addr string
}

// Init init conf
func Init() (err error) {
	if _, err = toml.DecodeFile(confPath, &Conf); err != nil {
		return
	}
	if schedulerPath != "" {
		Conf.Scheduler, _ = ioutil.ReadFile(schedulerPath)
	}
	return Conf.Fix()
}
