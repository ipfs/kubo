package inflect

import (
	"testing"
)

func TestPluralize(t *testing.T) {
	tests := []string{"half", "potato", "cello", "disco", "chef", "wife", "poppy", "sty", "football", "tester", "play", "hero", "tooth", "mouse", "goose", "person", "foot", "money", "monkey", "calf", "lie", "auto", "studio"}
	results := []string{"halves", "potatoes", "cellos", "discos", "chefs", "wives", "poppies", "sties", "footballs", "testers", "plays", "heroes", "teeth", "mice", "geese", "people", "feet", "money", "monkeys", "calves", "lies", "autos", "studios"}

	for index, test := range tests {
		if result := Pluralize(test); result != results[index] {
			t.Errorf("Expected %v, got %v", results[index], result)
		}
	}
}

func TestCommonPluralize(t *testing.T) {
	tests := []string{"user", "order", "product", "verse", "test", "upload", "class", "course", "game", "score", "body", "life", "dice"}
	results := []string{"users", "orders", "products", "verses", "tests", "uploads", "classes", "courses", "games", "scores", "bodies", "lives", "die"}

	for index, test := range tests {
		if result := Pluralize(test); result != results[index] {
			t.Errorf("Expected %v, got %v", results[index], result)
		}
	}
}

func TestSingularization(t *testing.T) {
	tests := []string{"halves", "potatoes", "cellos", "discos", "chefs", "wives", "poppies", "sties", "footballs", "testers", "plays", "heroes", "teeth", "mice", "geese", "people", "feet", "money", "monkeys", "calves", "lies", "autos", "studios"}
	results := []string{"half", "potato", "cello", "disco", "chef", "wife", "poppy", "sty", "football", "tester", "play", "hero", "tooth", "mouse", "goose", "person", "foot", "money", "monkey", "calf", "lie", "auto", "studio"}

	for index, test := range tests {
		if result := Singularize(test); result != results[index] {
			t.Errorf("Expected %v, got %v", results[index], result)
		}
	}
}

func TestCommonSingularization(t *testing.T) {
	tests := []string{"users", "orders", "products", "verses", "tests", "uploads", "classes", "courses", "games", "scores", "bodies", "lives", "die"}
	results := []string{"user", "order", "product", "verse", "test", "upload", "class", "course", "game", "score", "body", "life", "dice"}

	for index, test := range tests {
		if result := Singularize(test); result != results[index] {
			t.Errorf("Expected %v, got %v", results[index], result)
		}
	}
}

func TestUpperCamelCase(t *testing.T) {
	tests := []string{"_pre", "post_", "    spaced", "single", "lowerCamelCase", "under_scored", "hyphen-ated", "UpperCamelCase", "spaced Out"}
	results := []string{"Pre", "Post", "Spaced", "Single", "LowerCamelCase", "UnderScored", "HyphenAted", "UpperCamelCase", "SpacedOut"}

	for index, test := range tests {
		if result := UpperCamelCase(test); result != results[index] {
			t.Errorf("Expected %v, got %v", results[index], result)
		}
	}
}

func TestLowerCamelCase(t *testing.T) {
	tests := []string{"single", "lowerCamelCase", "under_scored", "hyphen-ated", "UpperCamelCase", "spaced Out"}
	results := []string{"single", "lowerCamelCase", "underScored", "hyphenAted", "upperCamelCase", "spacedOut"}

	for index, test := range tests {
		if result := LowerCamelCase(test); result != results[index] {
			t.Errorf("Expected %v, got %v", results[index], result)
		}
	}
}

func TestUnderscore(t *testing.T) {
	tests := []string{"single", "lowerCamelCase", "under_scored", "hyphen-ated", "UpperCamelCase", "spaced Out"}
	results := []string{"single", "lower_camel_case", "under_scored", "hyphen_ated", "upper_camel_case", "spaced_out"}

	for index, test := range tests {
		if result := Underscore(test); result != results[index] {
			t.Errorf("Expected %v, got %v", results[index], result)
		}
	}
}

func TestHyphenate(t *testing.T) {
	tests := []string{"single", "lowerCamelCase", "under_scored", "hyphen-ated", "UpperCamelCase", "spaced Out"}
	results := []string{"single", "lower-camel-case", "under-scored", "hyphen-ated", "upper-camel-case", "spaced-out"}

	for index, test := range tests {
		if result := Hyphenate(test); result != results[index] {
			t.Errorf("Expected %v, got %v", results[index], result)
		}
	}
}

func TestConstantize(t *testing.T) {
	tests := []string{"single", "lowerCamelCase", "under_scored", "hyphen-ated", "UpperCamelCase", "spaced Out"}
	results := []string{"SINGLE", "LOWER_CAMEL_CASE", "UNDER_SCORED", "HYPHEN_ATED", "UPPER_CAMEL_CASE", "SPACED_OUT"}

	for index, test := range tests {
		if result := Constantize(test); result != results[index] {
			t.Errorf("Expected %v, got %v", results[index], result)
		}
	}
}

func TestHumanize(t *testing.T) {
	tests := []string{"single", "lowerCamelCase", "under_scored", "hyphen-ated", "UpperCamelCase", "spaced Out"}
	results := []string{"Single", "Lower camel case", "Under scored", "Hyphen ated", "Upper camel case", "Spaced out"}

	for index, test := range tests {
		if result := Humanize(test); result != results[index] {
			t.Errorf("Expected %v, got %v", results[index], result)
		}
	}
}

func TestTitleize(t *testing.T) {
	tests := []string{"single", "lowerCamelCase", "under_scored", "hyphen-ated", "UpperCamelCase", "spaced Out"}
	results := []string{"Single", "Lower Camel Case", "Under Scored", "Hyphen Ated", "Upper Camel Case", "Spaced Out"}

	for index, test := range tests {
		if result := Titleize(test); result != results[index] {
			t.Errorf("Expected %v, got %v", results[index], result)
		}
	}
}
