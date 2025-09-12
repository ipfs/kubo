package commands

import "testing"

func TestScrubMapInternalDelete(t *testing.T) {
	m, err := scrubMapInternal(nil, nil, true)
	if err != nil {
		t.Error(err)
	}
	if m == nil {
		t.Errorf("expecting an empty map, got nil")
	}
	if len(m) != 0 {
		t.Errorf("expecting an empty map, got a non-empty map")
	}
}

func TestEditorParsing(t *testing.T) {
	testCases := []struct {
		name     string
		input    string
		expected []string
		hasError bool
	}{
		{
			name:     "simple editor",
			input:    "vim",
			expected: []string{"vim"},
			hasError: false,
		},
		{
			name:     "editor with single flag",
			input:    "emacs -nw",
			expected: []string{"emacs", "-nw"},
			hasError: false,
		},
		{
			name:     "VS Code with wait flag (issue #9375)",
			input:    "code --wait",
			expected: []string{"code", "--wait"},
			hasError: false,
		},
		{
			name:     "VS Code with full path and wait flag (issue #9375)",
			input:    "/opt/homebrew/bin/code --wait",
			expected: []string{"/opt/homebrew/bin/code", "--wait"},
			hasError: false,
		},
		{
			name:     "editor with quoted path containing spaces",
			input:    "\"/Applications/Visual Studio Code.app/Contents/Resources/app/bin/code\" --wait",
			expected: []string{"/Applications/Visual Studio Code.app/Contents/Resources/app/bin/code", "--wait"},
			hasError: false,
		},
		{
			name:     "sublime text with wait flag",
			input:    "subl -w",
			expected: []string{"subl", "-w"},
			hasError: false,
		},
		{
			name:     "nano editor",
			input:    "nano",
			expected: []string{"nano"},
			hasError: false,
		},
		{
			name:     "gedit editor",
			input:    "gedit",
			expected: []string{"gedit"},
			hasError: false,
		},
		{
			name:     "editor with multiple flags",
			input:    "vim -c 'set number' -c 'set hlsearch'",
			expected: []string{"vim", "-c", "set number", "-c", "set hlsearch"},
			hasError: false,
		},
		{
			name:     "trailing backslash (POSIX edge case)",
			input:    "editor\\",
			expected: nil,
			hasError: true,
		},
		{
			name:     "double quoted editor name with spaces",
			input:    "\"code with spaces\" --wait",
			expected: []string{"code with spaces", "--wait"},
			hasError: false,
		},
		{
			name:     "single quoted editor with flags",
			input:    "'my editor' -flag",
			expected: []string{"my editor", "-flag"},
			hasError: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result, err := parseEditorCommand(tc.input)

			if tc.hasError {
				if err == nil {
					t.Errorf("Expected error for input '%s', but got none", tc.input)
				}
				return
			}

			if err != nil {
				t.Errorf("Unexpected error for input '%s': %v", tc.input, err)
				return
			}

			if len(result) != len(tc.expected) {
				t.Errorf("Expected %d args, got %d for input '%s'", len(tc.expected), len(result), tc.input)
				t.Errorf("Expected: %v", tc.expected)
				t.Errorf("Got: %v", result)
				return
			}

			for i, expected := range tc.expected {
				if result[i] != expected {
					t.Errorf("Expected arg %d to be '%s', got '%s' for input '%s'", i, expected, result[i], tc.input)
				}
			}
		})
	}
}
