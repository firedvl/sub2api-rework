package service

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestProvideAccountTestServiceRetainsPluginAccountDirectory(t *testing.T) {
	gateway := &OpenAIGatewayService{}
	plugins := &PluginManager{}
	ProvideAccountTestService(nil, nil, nil, nil, nil, nil, nil, nil, gateway, nil, plugins)
	require.Same(t, gateway, plugins.accountDirectory)
}
