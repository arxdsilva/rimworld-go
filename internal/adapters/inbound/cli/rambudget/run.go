package rambudget

import (
	"fmt"
	"io"

	"rimworld-go/internal/platform/sysinfo"
)

// Main prints the budget to out, or the reason it cannot to errOut, and
// returns the exit code.
func Main(out, errOut io.Writer) int {
	i := sysinfo.Read()
	if i.Total == 0 {
		fmt.Fprintln(errOut, "rambudget: this platform does not report its memory; size agent batches by hand")
		return 1
	}
	print(out, i)
	return 0
}

func print(out io.Writer, i sysinfo.Info) {
	avail := int(i.Available >> 20)
	b := budget(avail, reserveMiB)
	fmt.Fprintf(out, "memory: %.1f GiB total, %.1f GiB free, %d cores\n", i.TotalGiB(), i.AvailableGiB(), i.Cores)
	fmt.Fprintf(out, "budget: %.1f GiB after a %.0f GiB reserve for the OS, editor and lead\n", float64(b.BudgetMiB)/1024, float64(reserveMiB)/1024)
	for _, k := range b.Kinds {
		fmt.Fprintf(out, "  %-7s up to %2d at once (peak %.1f GiB each: %s)\n", k.Name, k.Max, float64(k.PeakMiB)/1024, k.What)
	}
	fmt.Fprintln(out, "mixing kinds: keep the sum of their peaks under the budget; start heavy agents a few minutes apart; agents test with GOFLAGS=-p=2")
	if b.Kinds[0].Max == 0 {
		fmt.Fprintln(out, "too little free memory for even one heavy agent: work alone or close programs first")
	}
}
