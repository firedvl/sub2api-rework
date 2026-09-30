package claude

import "sync/atomic"

type cliVersionResolverFunc func() string

var cliVersionResolver atomic.Pointer[cliVersionResolverFunc]

func SetCLIVersionResolver(resolver func() string) {
	if resolver == nil {
		cliVersionResolver.Store(nil)
		return
	}
	resolverFunc := cliVersionResolverFunc(resolver)
	cliVersionResolver.Store(&resolverFunc)
}

func EffectiveCLIVersion() string {
	if resolver := cliVersionResolver.Load(); resolver != nil {
		if version := (*resolver)(); IsSupportedCLIVersion(version) {
			return version
		}
	}
	return CLIVersion()
}

func DefaultUserAgent() string {
	return "claude-cli/" + EffectiveCLIVersion() + " (external, cli)"
}
