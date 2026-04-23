package docs

import (
	"embed"
)

//go:embed sections/*.md
var SectionsFS embed.FS

func GetSection(name string) (string, error) {
	data, err := SectionsFS.ReadFile("sections/" + name + ".md")
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func ListSections() []string {
	entries, err := SectionsFS.ReadDir("sections")
	if err != nil {
		return nil
	}
	var names []string
	for _, e := range entries {
		names = append(names, e.Name()[:len(e.Name())-3])
	}
	return names
}

func AllContent() string {
	sections := ListSections()
	var result string
	for _, s := range sections {
		content, err := GetSection(s)
		if err != nil {
			continue
		}
		result += content + "\n\n"
	}
	return result
}
