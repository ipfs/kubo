package cli

import (
  "strings"
  "fmt"

  "github.com/jbenet/go-ipfs/commands"
)

func Parse(input []string, root *commands.Command) ([]string, []string, map[string]string, error) {
  path, input, err := parsePath(input, root)
  if err != nil {
    return nil, nil, nil, err
  }

  opts, args, err := parseOptions(input, path, root)
  if err != nil {
    return nil, nil, nil, err
  }

  return path, args, opts, nil
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
func parseOptions(input, path []string, root *commands.Command) (map[string]string, []string, error) {
  options, err := root.GetOptions(path)
  if err != nil {
    return nil, nil, err
  }

  opts := make(map[string]string)
  args := make([]string, 0)

  // TODO: error if one option is defined multiple times

  for i := 0; i < len(input); i++ {
    blob := input[i]

    if strings.HasPrefix(blob, "--") {
      name := blob[2:]
      value := ""

      if strings.Contains(name, "=") {
        split := strings.SplitN(name, "=", 2)
        name = split[0]
        value = split[1]
      }

      opts[name] = value

    } else if strings.HasPrefix(blob, "-") {
      blob = blob[1:]

      if strings.ContainsAny(blob, "-=\"") {
        return nil, nil, fmt.Errorf("Invalid option blob: '%s'", input[i])
      }

      nameS := ""
      for _, name := range blob {
        nameS = string(name)
        opts[nameS] = ""
      }

      if nameS != "" {
        opt, ok := options[nameS]
        if ok && opt.Type != commands.Bool {
          i++
          if i <= len(input) {
            opts[nameS] = input[i]
          }
        }
      }

    } else {
      args = append(args, blob)
    }
  }

  return opts, args, nil
}
