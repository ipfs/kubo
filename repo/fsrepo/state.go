package fsrepo

type state int

const (
	unopened = iota
	opened
	closed
)

func (s state) String() string {
	switch s {
	case unopened:
		return "unopened"
	case opened:
		return "opened"
	case closed:
		return "closed"
	default:
		return "invalid"
	}
}
