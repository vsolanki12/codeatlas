package create

type Reconciler struct {
	client Client
}

func (r *Reconciler) Reconcile() {
	secret := &Secret{}
	r.client.Create(secret)
	r.client.Create(&ConfigMap{})
}

type Client struct{}
type Secret struct{}
type ConfigMap struct{}

func (c Client) Create(object interface{}) {}
