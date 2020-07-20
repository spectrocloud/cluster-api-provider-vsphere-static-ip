package metal3io

import (
	"testing"

	ipamv1 "github.com/metal3-io/ip-address-manager/api/v1alpha1"
	"github.com/stretchr/testify/assert"
)

func TestConvertToIpamAddressStr(t *testing.T) {
	mIP := ipamv1.IPAddressStr("10.10.120.10")
	res := convertToIpamAddressStr(&mIP)
	assert.NotEmpty(t, res)
}
