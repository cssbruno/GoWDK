package view

func collectActionFormFields(nodes []Node, fields map[string]map[string]ActionFormField) error {
	for _, node := range nodes {
		switch typed := node.(type) {
		case Element:
			action, err := elementPostActionName(typed)
			if err != nil {
				return err
			}
			if action != "" {
				if fields[action] == nil {
					fields[action] = map[string]ActionFormField{}
				}
				if err := validateActionForm(typed); err != nil {
					return err
				}
				if err := collectNamedControls(typed.Children, fields[action]); err != nil {
					return err
				}
				continue
			}
			if err := collectActionFormFields(typed.Children, fields); err != nil {
				return err
			}
		case ComponentCall:
			if err := collectActionFormFields(typed.Children, fields); err != nil {
				return err
			}
		}
	}
	return nil
}
