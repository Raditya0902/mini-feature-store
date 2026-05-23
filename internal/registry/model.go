package registry

import (
	"fmt"
	"time"
)

type Duration struct{ time.Duration }

func (d *Duration) UnmarshalYAML(unmarshal func(any) error) error {
	var s string
	if err := unmarshal(&s); err != nil {
		return err
	}
	dur, err := time.ParseDuration(s)
	if err != nil {
		return fmt.Errorf("invalid duration %q: %w", s, err)
	}
	d.Duration = dur
	return nil
}

type Entity struct {
	Name        string `yaml:"name"`
	JoinKey     string `yaml:"join_key"`
	Description string `yaml:"description,omitempty"`
}

type Feature struct {
	Name  string `yaml:"name"`
	Dtype string `yaml:"dtype"`
}

type FeatureView struct {
	Name     string    `yaml:"name"`
	Entity   string    `yaml:"entity"`
	Source   string    `yaml:"source"`
	TTL      Duration  `yaml:"ttl"`
	Features []Feature `yaml:"features"`
}

type Registry struct {
	Entities     []Entity      `yaml:"entities"`
	FeatureViews []FeatureView `yaml:"feature_views"`
}
