package setup

import (
	"get.porter.sh/magefiles/porter"
	"github.com/magefile/mage/mg"
	"golang.org/x/sync/errgroup"
)

// InstallMixins used by the test bundles.
// If you add a test that uses a new mixin, update this function to install it.
func InstallMixins() error {
	mg.SerialDeps(porter.UseBinForPorterHome, porter.EnsurePorter)

	mixins := []porter.InstallMixinOptions{
		{Name: "exec"},
	}
	var errG errgroup.Group
	for _, mixin := range mixins {
		mixin := mixin
		errG.Go(func() error {
			return porter.EnsureMixin(mixin)
		})
	}
	return errG.Wait()
}
