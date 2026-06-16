package theme

// builtinThemes maps theme names to their constructors
var builtinThemes = map[string]func() Theme{
	"default":    DefaultTheme,
	"catppuccin": CatppuccinTheme,
	"dracula":    DraculaTheme,
	"tokyonight": TokyoNightTheme,
}

// GetTheme returns a theme by name. Falls back to "default" if not found.
func GetTheme(name string) Theme {
	if constructor, ok := builtinThemes[name]; ok {
		return constructor()
	}
	return DefaultTheme()
}

// AvailableThemes returns the names of all registered themes.
func AvailableThemes() []string {
	names := make([]string, 0, len(builtinThemes))
	for name := range builtinThemes {
		names = append(names, name)
	}
	return names
}
