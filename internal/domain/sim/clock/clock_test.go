package clock

import "testing"

func TestNewStartsAtZero(t *testing.T) {
	c := New()
	if got := c.Now(); got != 0 {
		t.Fatalf("Now() = %d, want 0", got)
	}
}

func TestAdvanceIncrementsTick(t *testing.T) {
	c := New()
	c.Advance(5)
	if got := c.Now(); got != 5 {
		t.Fatalf("Now() = %d, want 5", got)
	}
	c.Advance(1)
	if got := c.Now(); got != 6 {
		t.Fatalf("Now() = %d, want 6", got)
	}
}

func TestAdvanceIgnoresNonPositive(t *testing.T) {
	c := New()
	c.Advance(0)
	c.Advance(-3)
	if got := c.Now(); got != 0 {
		t.Fatalf("Now() = %d, want 0 (clock must never move backward)", got)
	}
}

func TestDateAtZero(t *testing.T) {
	got := DateAt(0)
	want := Date{Year: 0, Quadrum: 0, Day: 0, Hour: 0}
	if got != want {
		t.Fatalf("DateAt(0) = %+v, want %+v", got, want)
	}
}

func TestDateAtBoundaries(t *testing.T) {
	cases := []struct {
		name string
		tick Tick
		want Date
	}{
		{"one hour", TicksPerHour, Date{Hour: 1}},
		{"last hour of day", TicksPerDay - TicksPerHour, Date{Hour: HoursPerDay - 1}},
		{"one day", TicksPerDay, Date{Day: 1}},
		{"one quadrum", TicksPerQuadrum, Date{Quadrum: 1}},
		{"one year", TicksPerYear, Date{Year: 1}},
		{"mid-year", TicksPerYear + TicksPerQuadrum + TicksPerDay + TicksPerHour,
			Date{Year: 1, Quadrum: 1, Day: 1, Hour: 1}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := DateAt(c.tick); got != c.want {
				t.Fatalf("DateAt(%d) = %+v, want %+v", c.tick, got, c.want)
			}
		})
	}
}

func TestClockDateMatchesNow(t *testing.T) {
	c := New()
	c.Advance(TicksPerDay + TicksPerHour*3)
	want := DateAt(c.Now())
	if got := c.Date(); got != want {
		t.Fatalf("Date() = %+v, want %+v", got, want)
	}
}

func TestDateString(t *testing.T) {
	d := Date{Year: 2, Quadrum: 1, Day: 5, Hour: 14}
	want := "Year 3, Quadrum 2, Day 6, afternoon 14:00:00"
	if got := d.String(); got != want {
		t.Fatalf("String() = %q, want %q", got, want)
	}
}

func TestDateAtHalfHour(t *testing.T) {
	got := DateAt(TicksPerHour / 2)
	if got.Hour != 0 || got.Minute != 30 || got.Second != 0 {
		t.Fatalf("DateAt(half hour) = %+v, want Hour=0 Minute=30 Second=0", got)
	}
}

func TestSimSecondsMatchesTicksPerSecond(t *testing.T) {
	if SimSeconds(2*TicksPerSecond) != 2 {
		t.Fatalf("SimSeconds(2s) = %d", SimSeconds(2*TicksPerSecond))
	}
}

func TestPeriodAtHours(t *testing.T) {
	cases := []struct {
		hour int
		want Period
	}{
		{0, Night},
		{5, Night},
		{6, Morning},
		{11, Morning},
		{12, Afternoon},
		{16, Afternoon},
		{17, Evening},
		{19, Evening},
		{20, Night},
		{23, Night},
	}
	for _, tc := range cases {
		if got := PeriodAt(tc.hour); got != tc.want {
			t.Fatalf("PeriodAt(%d) = %s, want %s", tc.hour, got, tc.want)
		}
	}
}

func TestHourOfDayWraps(t *testing.T) {
	if got := HourOfDay(0); got != 0 {
		t.Fatalf("HourOfDay(0) = %v", got)
	}
	if got := HourOfDay(TicksPerHour); got != 1 {
		t.Fatalf("HourOfDay(1h) = %v", got)
	}
	if got := HourOfDay(TicksPerDay); got != 0 {
		t.Fatalf("HourOfDay(1d) = %v, want 0", got)
	}
}

func TestOneXRealTimeConversions(t *testing.T) {
	if TicksPerDay != 60_000 {
		t.Fatalf("TicksPerDay = %d, want 60000", TicksPerDay)
	}
	if DaysPerYear != 60 {
		t.Fatalf("DaysPerYear = %d, want 60", DaysPerYear)
	}
	sec := RealSecondsPerHour()
	if sec < 41.6 || sec > 41.7 {
		t.Fatalf("RealSecondsPerHour() = %v, want ~41.67", sec)
	}
}

func TestNewAtAndStartingMorning(t *testing.T) {
	c := NewAt(StartingMorning())
	d := c.Date()
	if d.Hour != StartHour || d.Period() != Morning {
		t.Fatalf("starting morning = %+v period %s", d, d.Period())
	}
}
