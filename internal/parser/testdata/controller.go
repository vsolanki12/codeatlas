package fake

type FakeReconciler struct{}

func (r *FakeReconciler) Reconcile() {}

func (r *FakeReconciler) SetupWithManager(mgr Manager) error {
	return NewControllerManagedBy(mgr).
		For(&HostedCluster{}).
		Owns(&Secret{}).
		Complete(r)
}

type Manager interface{}
type HostedCluster struct{}
type Secret struct{}

func NewControllerManagedBy(mgr Manager) *Builder { return &Builder{} }

type Builder struct{}

func (b *Builder) For(obj interface{}) *Builder { return b }
func (b *Builder) Owns(obj interface{}) *Builder { return b }
func (b *Builder) Complete(r interface{}) error  { return nil }
