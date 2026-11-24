package config

// ServiceConfig 声明服务自身标识（替代原 nacos.registry / nacos.config 段）。
//
// K8s/DNS 服务发现方案下，服务注册由平台承担（K8s Service DNS / compose 网络别名
// / extra_hosts），不再需要向注册中心注册。这里只保留两个用途：
//   - Name：服务标识，作为观测（tracing/metrics）的 service.name；
//   - ConfigName：业务环境配置文件 <config_name>-<env>.yaml 的推导基准，
//     默认取 Name。
//
// 灰度实例的注册名是 devops-system-gray，但业务配置与主实例共享
// （devops-system-dev.yaml），因此把 ConfigName 覆盖为 devops-system；
// 不覆盖时配置文件名会从灰度名推导（devops-system-gray-dev.yaml 不存在），
// 实例以 mysql.host must not be empty 启动失败。
type ServiceConfig struct {
	Name       string `mapstructure:"name"`
	ConfigName string `mapstructure:"config_name"`
}
