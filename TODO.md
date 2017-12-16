# TODO before merge

- t0110 fails (marked as known breakage for now)
- t0410 fails (marked as known breakage for now)
- errors in plugin loading are not fatal (but should be).
	On master the plugins are loaded after PreRun is called, but
	before calling PostRun or Run. I want to remove all these details
	from main.go using Executors, so we can only load before PreRun.

	One sharness test case however (at the beginning of t0020)
	expects that when the ipfs repo is not accessible, the error
	in the `init` commands' PreRun is printed. Instead, the error
	printed is the one from loading the plugin.

	Not sure how to resolve this the best way.
