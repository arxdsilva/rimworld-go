package ebiten

// Input layer (UI-016): the only place the client reads Ebitengine's
// keyboard and mouse. inputSource polls once per App.Update into a
// rawInput; App.pollInput resolves it against the bindings and the
// context stack into the frame every handler reads (input_frame.go).
// Tests swap in a fake source.
