package stdlib

// Modules currently represented by catalog declarations. Further modules can be
// introduced alongside their declarations without changing binder lookup rules.
var modules = [...]Module{
	{Name: "ColorConstants", Section: "6.1.2.1"},
	{Name: "Constants", Section: "6.1.2.2"},
	{Name: "Conversion", Section: "6.1.2.3"},
	{Name: "Information", Section: "6.1.2.7"},
	{Name: "Interaction", Section: "6.1.2.8"},
	{Name: "Strings", Section: "6.1.2.11"},
	{Name: "KeyCodeConstants", Section: "6.1.2.9"},
	{Name: "SystemColorConstants", Section: "6.1.2.12"},
}
