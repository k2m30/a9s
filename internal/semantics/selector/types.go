package selector

// TagOperator specifies the comparison operator for a tag condition.
type TagOperator int

const (
	TagEquals    TagOperator = iota // tag value equals condition Value
	TagNotEquals                    // tag value does not equal condition Value
	TagExists                       // tag key is present (Value ignored)
	TagNotExists                    // tag key is absent (Value ignored)
)

// TagCondition specifies a single predicate on a resource tag.
type TagCondition struct {
	Key      string
	Operator TagOperator
	Value    string
}
