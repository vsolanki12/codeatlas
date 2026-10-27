package fake

// TargetConfigController keeps a target config in sync.
type TargetConfigController struct {
	client Client
}

func NewTargetConfigController(recorder Recorder) Controller {
	c := &TargetConfigController{}
	return NewFactory().
		WithInformers(configMapInformer()).
		WithSync(c.sync).
		ResyncEvery(0).
		ToController("TargetConfigController", recorder)
}

func (c *TargetConfigController) sync() error {
	validateTargetConfig()
	return nil
}

// RevocationController registers a sync method whose name is not "sync".
type RevocationController struct{}

func NewRevocationController(recorder Recorder) Controller {
	c := &RevocationController{}
	return NewFactory().WithSync(c.syncRevocationRequest).ToController("RevocationController", recorder)
}

func (c *RevocationController) syncRevocationRequest() error { return nil }

// NotAController never reaches ToController, so no controller is created.
func NotAController() *Factory {
	c := &Unrelated{}
	return NewFactory().WithSync(c.sync)
}

type Unrelated struct{}

func (u *Unrelated) sync() error { return nil }

type Controller interface{}
type Recorder struct{}
type Factory struct{}

func NewFactory() *Factory                                      { return &Factory{} }
func (f *Factory) WithInformers(i ...any) *Factory              { return f }
func (f *Factory) WithSync(fn any) *Factory                     { return f }
func (f *Factory) ResyncEvery(d int) *Factory                   { return f }
func (f *Factory) ToController(n string, r Recorder) Controller { return nil }

func configMapInformer() any {}
func validateTargetConfig()  {}
