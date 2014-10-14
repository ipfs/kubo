package cli

import (
  "strings"
  "fmt"

  "github.com/jbenet/go-ipfs/commands"
)

func Parse(input []string, root *commands.Command) ([]string, []string, map[string]string, error) {
  opts, input, err := options(input, root)
  if err != nil {
    return nil, nil, nil, err
  }

  path, args, err := path(input, root)
  if err != nil {
    return nil, nil, nil, err
  }

  return path, args, opts, nil
}

// options parses the raw string values of the given options
// returns the parsed options as strings, along with the CLI input minus option blobs
func options(input []string, root *commands.Command) (map[string]string, []string, error) {
  opts := make(map[string]string)
  cleanInput := make([]string, 0)

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

      if strings.Contains(name, "-") {
        return nil, nil, fmt.Errorf("Invalid option blob: '%s' (Shouldn't contain '-')", input[i])
      }

      if value != "" && strings.Contains(value, "\"") {
        // TODO: ignore escaped quotations (--foo="\"")
        if !strings.HasPrefix(value, "\"") {
          return nil, nil, fmt.Errorf("Invalid option blob: '%s' (Quotation wasn't at the start of value)", input[i])
        }

        value = value[1:]

        for {
          if strings.HasSuffix(value, "\"") {
            value = value[:len(value)-1]
            break
          }

          i++
          if i >= len(input) {
            return nil, nil, fmt.Errorf("Unterminated string: '%s'", value)
          }

          value += " " + input[i]
        }

        if strings.Contains(value, "\"") {
          return nil, nil, fmt.Errorf("Invalid option blob: '%s' (Value contains unescaped quotation)", value)
        }
      }

      opts[name] = value

    } else if strings.HasPrefix(blob, "-") {
      blob = blob[1:]

      if strings.ContainsAny(blob, "-=\"") {
        return nil, nil, fmt.Errorf("Invalid option blob: '%s'", input[i])
      }

      for _, name := range blob {
        opts[string(name)] = ""
      }

      // TODO: interpret next blob as value if the last option isn't a bool

    } else {
      cleanInput = append(cleanInput, blob)
    }
  }

  return opts, cleanInput, nil
}

// path takes the command line (without options) and splits it into the command path and arguments
func path(input []string, root *commands.Command) ([]string, []string, error) {
  cmd := root
  i := 0

  for _, blob := range input {
    cmd := cmd.Sub(blob)

    if cmd == nil {
      break
    }

    i++
  }

  return input[:i], input[i:], nil
}
