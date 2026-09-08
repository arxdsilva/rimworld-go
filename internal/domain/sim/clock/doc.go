// Package clock is the authoritative simulation calendar: integer ticks,
// dates, and derived day periods (morning, afternoon, evening, night).
//
// 1x play maps TicksPerSecond ticks to one real second. Hours, days, and
// years are derived from that tick (2500 ticks/hour, 24-hour days,
// 60-day years). Clock-face MM:SS maps that hour onto 60 minutes; it is
// not 60 simulation seconds per minute. Game speed only changes how
// many ticks the renderer steps.
package clock
