package cli

import (
  "strings"

  "github.com/jbenet/go-ipfs/commands"
)

func Parse(input []string, root *commands.Command) (*commands.Request, error) {
  path, input, err := parsePath(input, root)
  if err != nil {
    return nil, err
  }

  opts, args, err := parseOptions(input)
  if err != nil {
    return nil, err
  }

  return commands.NewRequest(path, opts, args), nil
}

// parsePath gets the command path from the command line input
func parsePath(input []string, root *commands.Command) ([]string, []string, error) {
  cmd := root
  i := 0

  for _, blob := range input {
    if strings.HasPrefix(blob, "-") {
      break
    }

    cmd := cmd.Sub(blob)
    if cmd == nil {
      break
    }

    i++
  }

  return input[:i], input[i:], nil
}

// parseOptions parses the raw string values of the given options
// returns the parsed options as strings, along with the CLI args
func parseOptions(input []string) (map[string]interface{}, []string, error) {
  opts := make(map[string]interface{})
  args := make([]string, 0)

  // TODO: error if one option is defined multiple times

  for i := 0; i < len(input); i++ {
    blob := input[i]

    if strings.HasPrefix(blob, "-") {
      name := blob[1:]
      value := ""

      // support single and double dash
      if strings.HasPrefix(name, "-") {
        name = name[1:]
      }

      if strings.Contains(name, "=") {
        split := strings.SplitN(name, "=", 2)
        name = split[0]
        value = split[1]
      }

      opts[name] = value

    } else {
      args = append(args, blob)
    }
  }

  return opts, args, nil
}
