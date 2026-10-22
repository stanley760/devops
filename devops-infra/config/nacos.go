package config

import "time"

type NacosConfig struct {
	Enabled   bool          `mapstructure:"enabled"`   // master switch; the whole package is a no-op when false
	Endpoints []string      `mapstructure:"endpoints"` // ["127.0.0.1:8848"]
	Namespace string        `mapstructure:"namespace"` // devops
	Group     string        `mapstructure:"group"`     // DEVOPS_GROUP
	Username  string        `mapstructure:"username"`
	Password  string        `mapstructure:"password"`
	Timeout   time.Duration `mapstructure:"timeout"`

	ConfigCenter ConfigCenterConfig `mapstructure:"config"`   // data_ids/required
	Registry     RegistryConfig     `mapstructure:"registry"` // service_name/port/weight
}

type ConfigCenterConfig struct {
	DataIDs  []string      `mapstructure:"data_ids"`
	Group    string        `mapstructure:"group"`
	Timeout  time.Duration `mapstructure:"timeout"`
	Required bool          `mapstructure:"required"`
	// ServiceName overrides the config-side service name used to derive the dataID
	// (<name>.yaml / <name>-<env>.yaml). It defaults to nacos.registry.service_name.
	// The gray instance registers as "devops-system-gray" but still reads the main
	// instance's business config ("devops-system" / "devops-system-dev.yaml"),
	// so it must be set explicitly — otherwise the config file name is derived from
	// the gray name and the instance cannot find any configuration (mysql.host empty).
	ServiceName string `mapstructure:"service_name"`
}

type RegistryConfig struct {
	ServiceName string            `mapstructure:"service_name"`
	Cluster     string            `mapstructure:"cluster"`
	Weight      float64           `mapstructure:"weight"`
	Meta        map[string]string `mapstructure:"meta"`
}
