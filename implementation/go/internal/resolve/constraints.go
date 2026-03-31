package resolve

import constraintspkg "github.com/artur-ciocanu/project-cumasach/implementation/go/internal/constraints"

type Constraint = constraintspkg.Constraint

func ParseConstraint(raw string) (Constraint, error) {
	return constraintspkg.ParseConstraint(raw)
}

func MergeConstraints(raws ...string) (Constraint, error) {
	return constraintspkg.MergeConstraints(raws...)
}
