package nestedcalls

type Reconciler struct {
	RegistryProvider interface{ Reconcile() }
}

func (r *Reconciler) Reconcile() {}

func (r *Reconciler) reconcile() {
	r.RegistryProvider.Reconcile()
}
