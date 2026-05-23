package registry

import "fmt"

func Validate(reg *Registry) error {
	if err := validateEntities(reg.Entities); err != nil {
		return err
	}
	return validateFeatureViews(reg.FeatureViews, entityNameSet(reg.Entities))
}

func validateEntities(entities []Entity) error {
	seen := make(map[string]bool)
	for _, e := range entities {
		if e.Name == "" {
			return fmt.Errorf("entity name is required")
		}
		if e.JoinKey == "" {
			return fmt.Errorf("entity %q: join_key is required", e.Name)
		}
		if seen[e.Name] {
			return fmt.Errorf("duplicate entity name %q", e.Name)
		}
		seen[e.Name] = true
	}
	return nil
}

func entityNameSet(entities []Entity) map[string]bool {
	names := make(map[string]bool, len(entities))
	for _, e := range entities {
		names[e.Name] = true
	}
	return names
}

func validateFeatureViews(views []FeatureView, knownEntities map[string]bool) error {
	seen := make(map[string]bool)
	for _, fv := range views {
		if fv.Name == "" {
			return fmt.Errorf("feature view name is required")
		}
		if fv.Entity == "" {
			return fmt.Errorf("feature view %q: entity is required", fv.Name)
		}
		if fv.Source == "" {
			return fmt.Errorf("feature view %q: source is required", fv.Name)
		}
		if fv.TTL.Duration <= 0 {
			return fmt.Errorf("feature view %q: ttl must be positive", fv.Name)
		}
		if len(fv.Features) == 0 {
			return fmt.Errorf("feature view %q: features must not be empty", fv.Name)
		}
		if seen[fv.Name] {
			return fmt.Errorf("duplicate feature view name %q", fv.Name)
		}
		seen[fv.Name] = true
		if !knownEntities[fv.Entity] {
			return fmt.Errorf("feature view %q: unknown entity %q", fv.Name, fv.Entity)
		}
	}
	return nil
}
