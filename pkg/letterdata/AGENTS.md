# pkg/letterdata

This package owns the game-independent persisted envelope for event letters.

Field is a typed name/value parameter. Code(title, format) gives an
older/generic letter a stable event identity from its original untranslated
template; named game events provide their explicit code. Defaults clones
caller-supplied fields or derives numbered fields from message arguments.
The owning letter stack controls these fields after posting.

Keep this package headless, dependency-free beyond the standard library,
free of runtime globals, and compatible with old gob records whose event
code and fields are absent. Domain event names belong to the corresponding
application owner, not this generic package.

Tests: TestDefaultsAndTypedArgs and application-level letter save/load,
old-save migration and read-model isolation.
