package viewrender

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
				multipart, err := validateActionForm(typed)
				if err != nil {
					return err
				}
				if err := collectNamedControls(typed.Children, fields[action], multipart); err != nil {
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
		case AwaitBlock:
			if err := collectActionFormFields(typed.Pending, fields); err != nil {
				return err
			}
			if err := collectActionFormFields(typed.Then, fields); err != nil {
				return err
			}
			if err := collectActionFormFields(typed.Catch, fields); err != nil {
				return err
			}
		}
	}
	return nil
}
