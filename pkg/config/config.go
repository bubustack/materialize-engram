package config

type Config struct{}

type Inputs struct {
	Mode     string         `mapstructure:"mode" json:"mode"`
	Template any            `mapstructure:"template" json:"template"`
	Vars     map[string]any `mapstructure:"vars" json:"vars"`
}
