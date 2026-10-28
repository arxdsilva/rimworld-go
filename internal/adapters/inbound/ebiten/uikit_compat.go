package ebiten

import (
	"image/color"

	ebiten "github.com/hajimehoshi/ebiten/v2"

	"rimworld-go/internal/adapters/inbound/ebiten/inputframe"
	"rimworld-go/internal/adapters/inbound/ebiten/uikit"
	"rimworld-go/internal/adapters/inbound/ebiten/wizard"
)

// The widget kit lives in uikit (ARCH-003). These aliases and one-line
// wrappers keep the root's call sites unchanged while its features move
// out one package at a time; each mover repoints its own files to uikit
// and the last one deletes this file (SOLID_GO.md §11). The App's own
// side of the kit (binding, the screen dispatch, the modal window) is in
// ui_state.go and ui_modal.go.

type (
	uiState       = uikit.Kit
	uiWidget      = uikit.Widget
	btnStyle      = uikit.BtnStyle
	btnOpts       = uikit.BtnOpts
	button        = uikit.Button
	buttonSpec    = uikit.ButtonSpec
	checkOpts     = uikit.CheckOpts
	radioOpts     = uikit.RadioOpts
	listSpec      = uikit.ListSpec
	modalButton   = uikit.ModalButton
	scrollArea    = uikit.ScrollArea
	scrollOpts    = uikit.ScrollOpts
	sliderSpec    = uikit.SliderSpec
	tableCol      = uikit.TableCol
	tabsOpts      = uikit.TabsOpts
	textBuf       = uikit.TextBuf
	textFieldSpec = uikit.TextFieldSpec
	tipCard       = uikit.TipCard
	textSpan      = uikit.TextSpan
	formCtx       = uikit.Form
)

const (
	btnDanger    = uikit.BtnDanger
	btnNormal    = uikit.BtnNormal
	btnPrimary   = uikit.BtnPrimary
	checkGap     = uikit.CheckGap
	dialogPad    = uikit.DialogPad
	fieldPad     = uikit.FieldPad
	footerBtnH   = uikit.FooterBtnH
	listRowH     = uikit.ListRowH
	minSetupH    = uikit.MinSetupH
	minSetupW    = uikit.MinSetupW
	modalCloseW  = uikit.ModalCloseW
	modalEdge    = uikit.ModalEdge
	modalFooterH = uikit.ModalFooterH
	modalPad     = uikit.ModalPad
	modalTitleH  = uikit.ModalTitleH
	radioRadius  = uikit.RadioRadius
	radioRowH    = uikit.RadioRowH
	scrollBarW   = uikit.ScrollBarW
	scrollStep   = uikit.ScrollStep
	sparkPad     = uikit.SparkPad
	tableCellPad = uikit.TableCellPad
	tableHeaderH = uikit.TableHeaderH
	tipDelay     = uikit.TipDelay
	tipMargin    = uikit.TipMargin
	tipMaxW      = uikit.TipMaxW
)

var (
	uiAccent        = uikit.Accent
	uiAlertDangerBg = uikit.AlertDangerBg
	uiAlertWarnBg   = uikit.AlertWarnBg
	uiBtn           = uikit.Btn
	uiBtnBorder     = uikit.BtnBorder
	uiBtnDisabled   = uikit.BtnDisabled
	uiBtnHover      = uikit.BtnHover
	uiBtnSelected   = uikit.BtnSelected
	uiDanger        = uikit.Danger
	uiDim           = uikit.Dim
	uiDust          = uikit.Dust
	uiField         = uikit.Field
	uiFieldBorder   = uikit.FieldBorder
	uiGood          = uikit.Good
	uiHeading       = uikit.Heading
	uiLetterCaution = uikit.LetterCaution
	uiLetterGood    = uikit.LetterGood
	uiLetterInfo    = uikit.LetterInfo
	uiLetterThreat  = uikit.LetterThreat
	uiMoodBad       = uikit.MoodBad
	uiMoodGood      = uikit.MoodGood
	uiMoodMid       = uikit.MoodMid
	uiPanel         = uikit.Panel
	uiPanelBorder   = uikit.PanelBorder
	uiPlanEdge      = uikit.PlanEdge
	uiPlanFill      = uikit.PlanFill
	uiPowerNet1     = uikit.PowerNet1
	uiPowerNet2     = uikit.PowerNet2
	uiPowerNet3     = uikit.PowerNet3
	uiPowerNet4     = uikit.PowerNet4
	uiPowerNet5     = uikit.PowerNet5
	uiPowerNet6     = uikit.PowerNet6
	uiPowerNone     = uikit.PowerNone
	uiScrollTrack   = uikit.ScrollTrack
	uiSliderFill    = uikit.SliderFill
	uiSliderTrack   = uikit.SliderTrack
	uiSpace         = uikit.Space
	uiText          = uikit.Text
	uiTextDim       = uikit.TextDim
	uiTextDisabled  = uikit.TextDisabled
	uiTokens        = uikit.Tokens
	uiTooltip       = uikit.Tooltip
	uiTooltipEdge   = uikit.TooltipEdge
	uiTutorBorder   = uikit.TutorBorder
	uiTutorHeading  = uikit.TutorHeading
	uiTutorPanel    = uikit.TutorPanel
	uiTutorText     = uikit.TutorText
	uiWarning       = uikit.Warning
)

// uiHost is the App as the kit's host.
type uiHost struct{ a *App }

var _ uikit.Host = uiHost{}

func (h uiHost) Frame() *inputframe.Frame  { return &h.a.in }
func (h uiHost) Cursor() (int, int)        { return h.a.currentCursor() }
func (h uiHost) Size() (int, int)          { return h.a.size() }
func (h uiHost) Updates() uint64           { return h.a.updates }
func (h uiHost) KeyRepeat(key string) bool { return h.a.keyRepeat(key) }
func (h uiHost) Cue(id string)             { h.a.cfg.Sound.Play(id, 1, 0) }
func (h uiHost) Hint(text string)          { h.a.hint = text }
func (h uiHost) ReducedMotion() bool       { return h.a.settings.Accessibility.ReducedMotion }

// ---- free functions

func asButton(r rect, id, label string) button                { return uikit.AsButton(r, id, label) }
func buttonColors(b button, hover bool) (f, bd, l color.RGBA) { return uikit.ButtonColors(b, hover) }
func debugPrintAt(dst *ebiten.Image, s string, x, y int)      { uikit.DebugPrintAt(dst, s, x, y) }
func dialogFrame(sw, sh int) (frame, title, content, footer rect) {
	return uikit.DialogFrame(sw, sh)
}
func digitsOnly(r rune) bool                             { return uikit.DigitsOnly(r) }
func drawButton(dst *ebiten.Image, b button, hover bool) { uikit.DrawButton(dst, b, hover) }
func drawCentered(dst *ebiten.Image, s string, cx, cy float64, st textStyle) {
	uikit.DrawCentered(dst, s, cx, cy, st)
}
func drawFitSmall(dst *ebiten.Image, s string, x, y, maxW float64, st textStyle) {
	uikit.DrawFitSmall(dst, s, x, y, maxW, st)
}
func drawLabel(dst *ebiten.Image, x, y int, text string) { uikit.DrawLabel(dst, x, y, text) }
func drawLabelClip(dst *ebiten.Image, x, y, maxW int, text string) {
	uikit.DrawLabelClip(dst, x, y, maxW, text)
}
func drawListRow(dst *ebiten.Image, row rect, label string, hover, selected, disabled bool) {
	uikit.DrawListRow(dst, row, label, hover, selected, disabled)
}
func drawMenuBackdrop(dst *ebiten.Image, sw, sh int)  { uikit.DrawMenuBackdrop(dst, sw, sh) }
func drawPanel(dst *ebiten.Image, x, y, w, h float32) { uikit.DrawPanel(dst, x, y, w, h) }
func drawProgress(dst *ebiten.Image, r rect, label string, frac float64, phase string) {
	uikit.DrawProgress(dst, r, label, frac, phase)
}
func drawSparkline(dst *ebiten.Image, r rect, vals []int, c color.Color) {
	uikit.DrawSparkline(dst, r, vals, c)
}
func drawTooSmall(dst *ebiten.Image, sw, sh int)        { uikit.DrawTooSmall(dst, sw, sh) }
func fieldInner(r rect) rect                            { return uikit.FieldInner(r) }
func fillRect(dst *ebiten.Image, r rect, c color.Color) { uikit.FillRect(dst, r, c) }
func fillTri(dst *ebiten.Image, x0, y0, x1, y1, x2, y2 float32) {
	uikit.FillTri(dst, x0, y0, x1, y1, x2, y2)
}
func fitText(s string, st textStyle, w float32) string      { return uikit.FitText(s, st, w) }
func footerBack(footer rect) rect                           { return uikit.FooterBack(footer) }
func footerNext(footer rect) rect                           { return uikit.FooterNext(footer) }
func hitButton(buttons []button, mx, my int) (button, bool) { return uikit.HitButton(buttons, mx, my) }
func layoutLines(rs []rune, st textStyle, maxW float64) []textSpan {
	return uikit.LayoutLines(rs, st, maxW)
}
func lerpRGBA(a, b color.RGBA, t float64) color.RGBA { return uikit.LerpRGBA(a, b, t) }
func listVisible(off, viewH, rowH float32, n int) (first, last int) {
	return uikit.ListVisible(off, viewH, rowH, n)
}
func nearestIndex(rs []rune, x float64) int { return uikit.NearestIndex(rs, x) }
func optID(id string, i int) string         { return uikit.OptID(id, i) }
func scrollThumb(bar rect, off, maxOff, contentH float32) rect {
	return uikit.ScrollThumbRect(bar, off, maxOff, contentH)
}
func sliderTrack(r rect) rect                     { return uikit.SliderTrackRect(r) }
func sparkPoints(vals []int, r rect) [][2]float32 { return uikit.SparkPoints(vals, r) }
func stackButtons(x, y, w, h, gap float32, specs []buttonSpec) []button {
	return uikit.StackButtons(x, y, w, h, gap, specs)
}
func strokeRect(dst *ebiten.Image, r rect, c color.Color) { uikit.StrokeRect(dst, r, c) }
func tabRects(r rect, labels []string) []rect             { return uikit.TabRects(r, labels) }
func tipLayout(tip string, sw, sh, mx, my int) tipCard    { return uikit.TipLayout(tip, sw, sh, mx, my) }
func tooSmall(sw, sh int) bool                            { return uikit.TooSmall(sw, sh) }
func uiFade(c color.RGBA, a uint8) color.RGBA             { return uikit.Fade(c, a) }
func uiNum(n int) string                                  { return uikit.Num(n) }
func uiShade(a uint8) color.RGBA                          { return uikit.Shade(a) }

// ---- the setup-screen composites (uikit/form.go)

func newForm(area scrollArea) *formCtx { return uikit.NewForm(area) }
func dimText() textStyle               { return uikit.DimText() }

// Kit is the App's widget kit (uikit.Owner: form rows take it).
func (a *App) Kit() *uikit.Kit { return a.uiSt() }

func (a *App) ngButton(id string, r rect, label string, o btnOpts, fn func()) {
	uikit.StyledButton(a.uiSt(), id, r, label, o, fn)
}
func (a *App) ngChoice(id string, r rect, title string, opts []string, cur int, disabled string, pick func(i int)) {
	uikit.Choice(a.uiSt(), id, r, title, opts, cur, disabled, pick)
}
func (a *App) pushPicker(id, title string, opts []string, cur int, pick func(i int)) {
	uikit.Picker(a.uiSt(), id, title, opts, cur, pick)
}
func (a *App) ngStepper(id string, r rect, v *int, lo, hi int) {
	uikit.Stepper(a.uiSt(), id, r, v, lo, hi)
}
func (a *App) rowLabel(r rect, label string, labelW float32) rect {
	return uikit.RowLabel(a.uiSt(), r, label, labelW)
}
func (a *App) segmented(id string, r rect, labels []string, sel *int, tips []string) {
	uikit.Segmented(a.uiSt(), id, r, labels, sel, tips)
}
func (a *App) textLines(x, y, w float32, s string, st textStyle) float32 {
	return uikit.TextLines(a.uiSt(), x, y, w, s, st)
}
func (a *App) declareOnScreenLayer() { uikit.OnScreenLayer(a.uiSt()) }

// diffSlider is a labelled slider row over *v with the range of the
// difficulty field key; a reason disables it.
func (a *App) diffSlider(f *formCtx, id, label string, v *int, key string, format func(int) string, disabled, tip string) {
	f.Slider(a, id, label, v, a.withTick(wizard.DiffSpec(key)), format, disabled, tip)
}
func (a *App) diffCheck(f *formCtx, id, label string, v *bool, disabled, tip string) {
	f.Check(a, id, label, v, disabled, tip)
}
func (a *App) diffStepper(f *formCtx, id, label string, v *int, lo, hi int, tip string) {
	f.Stepper(a, id, label, v, lo, hi, tip)
}

// ---- the kit's methods, on the App

// withTick gives a slider the App's step sound (sound_director.go).
func (a *App) withTick(sp sliderSpec) sliderSpec {
	if sp.Tick == nil {
		sp.Tick = func(lo, hi, old, cur float64) { a.sliderCue(sp, lo, hi, old, cur) }
	}
	return sp
}

func (a *App) uiButton(id string, r rect, label string, o btnOpts) bool {
	return a.uiSt().Button(id, r, label, o)
}
func (a *App) uiCheck(id string, r rect, label string, val *bool, o checkOpts) bool {
	return a.uiSt().Check(id, r, label, val, o)
}
func (a *App) uiClick(id string) bool { return a.uiSt().Click(id) }
func (a *App) uiConfirm(id, title, body, okLabel string, danger bool, onOK func()) {
	a.uiSt().Confirm(id, title, body, okLabel, danger, onOK)
}
func (a *App) uiDeny(reason string)                         { a.uiSt().Deny(reason) }
func (a *App) uiFocused(id string) bool                     { return a.uiSt().IsFocused(id) }
func (a *App) uiHot(id string) bool                         { return a.uiSt().IsHot(id) }
func (a *App) uiKeyAct(id string) bool                      { return a.uiSt().KeyAct(id) }
func (a *App) uiLabel(x, y float32, s string, st textStyle) { a.uiSt().Label(x, y, s, st) }
func (a *App) uiList(id string, r rect, n int, sel *int, sp listSpec) bool {
	return a.uiSt().List(id, r, n, sel, sp)
}
func (a *App) uiModalOpen() bool         { return a.uiSt().ModalOpen() }
func (a *App) uiOK()                     { a.uiSt().OK() }
func (a *App) uiPaint(dst *ebiten.Image) { a.uiSt().Paint(dst) }
func (a *App) uiPanel(r rect)            { a.uiSt().Panel(r) }
func (a *App) uiPress(r rect) bool       { return a.uiSt().Press(r) }
func (a *App) uiRadio(id string, r rect, labels []string, sel *int, o radioOpts) bool {
	return a.uiSt().Radio(id, r, labels, sel, o)
}
func (a *App) uiScrollArea(id string, r rect, contentH float32, o scrollOpts) scrollArea {
	return a.uiSt().ScrollArea(id, r, contentH, o)
}
func (a *App) uiScrollReveal(id string, viewH, y0, y1 float32) {
	a.uiSt().ScrollReveal(id, viewH, y0, y1)
}
func (a *App) uiSlider(id string, r rect, val *float64, sp sliderSpec) bool {
	return a.uiSt().Slider(id, r, val, a.withTick(sp))
}
func (a *App) uiSliderInt(id string, r rect, val *int, sp sliderSpec) bool {
	return a.uiSt().SliderInt(id, r, val, a.withTick(sp))
}
func (a *App) uiTable(id string, r rect, cols []tableCol, n int, sel *int, rowH float32) bool {
	return a.uiSt().Table(id, r, cols, n, sel, rowH)
}
func (a *App) uiTableSetSort(id string, col int, desc bool) { a.uiSt().TableSetSort(id, col, desc) }
func (a *App) uiTableSort(id string) (col int, desc bool)   { return a.uiSt().TableSort(id) }
func (a *App) uiTabs(id string, r rect, labels []string, sel *int, o tabsOpts) bool {
	return a.uiSt().Tabs(id, r, labels, sel, o)
}
func (a *App) uiTextEditing() bool { return a.uiSt().Editing() }
func (a *App) uiTextField(id string, r rect, val *string, sp textFieldSpec) (changed, committed bool) {
	return a.uiSt().TextField(id, r, val, sp)
}
func (a *App) uiTextInput()                        { a.uiSt().TextInput() }
func (a *App) uiTip(id string, r rect, tip string) { a.uiSt().Tip(id, r, tip) }
func (a *App) uiWheelOver(mx, my int) bool         { return a.uiSt().WheelOver(mx, my) }
func (a *App) setupTooSmall(sw, sh int) bool       { return a.uiSt().SetupTooSmall(sw, sh) }
func (a *App) tableOrder(id string, cols []tableCol, n int) []int {
	return a.uiSt().TableOrder(id, cols, n)
}
func (a *App) textEnd(commit bool) { a.uiSt().TextEnd(commit) }
