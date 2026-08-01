package mycomp

type ComponentOptions interface {
	IsRequestServing() bool
	MultiZoneSpread() bool
}

type Reconciler interface {
	Reconcile() error
}

type myComponent struct{}

var _ ComponentOptions = &myComponent{}

func (m *myComponent) IsRequestServing() bool { return false }
func (m *myComponent) MultiZoneSpread() bool  { return true }

type otherStruct struct{}

var _ Reconciler = (*otherStruct)(nil)
