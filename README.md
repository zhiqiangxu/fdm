# fdm, a general fork detector and manager

## usage

You're supposed to define 3 functions: `SnapFunc`,`PreStateFunc`,`ReorgFunc`, then just call `fdm.Submit(snapshot)` for every `header` and you're ready to go.

It's lossy only under 2 conditions:
1. the snapshot is not continuous
2. the fork is too long.(longer than `maxFork` to be exact)

Otherwise this module is supposed to seamlessly reorg the state as necessary.

Terms:

- `snapshot`: a header together with changed state before(`preState`) and after(`afterState`).
- `SnapFunc`: a user defined function to fetch snapshot by header hash, note the snapshot should only contain `afterState`.
- `PreStateFunc`: a user defined function to calculate `preState` by `afterState`.
- `ReorgFunc`: a user defined function to rollback old states and apply new states.