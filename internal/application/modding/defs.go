package modding

// CollectDefs reads p's defs files into trees (PLG-020): each file is
// {"<class>": [ {def}, ... ]}; a def has "name" (key "<plugin>.<name>"),
// optional "abstract" and "parent", and its fields. Owner: W1-patch.
func CollectDefs(p Plugin) ([]Def, []Problem) { return nil, nil }
