#!/usr/bin/env bash
# Compares the renderer builds on the cmd/renderer benchmarks
# (cmd/renderer/frame_bench_test.go):
#
#   default  no build tags: master's renderer (render_legacy.go), the baseline
#   scalar   -tags simd: new geometry and rasterizer, scalar kernels
#   simd     -tags simd with GOEXPERIMENT=simd: the same with SIMD kernels
#
# Every build is compiled once. Each round then runs every build's benchmarks
# once, reversing the build order every other round, so slow drift (heat,
# background load) is shared by all builds instead of landing on whichever ran
# last. Runs are pinned to one logical CPU. benchstat prints the median of each
# benchmark with a 95% confidence interval, and the change of scalar and simd
# against default with a p-value ("~" means no significant difference).
#
# Settings, as environment variables:
#   ROUNDS=10           samples per benchmark and build (benchstat wants >= 10)
#   BENCH=.             benchmark regexp, e.g. BENCH=Frame or BENCH='Frame/view=default'
#   BENCHTIME=1s        -test.benchtime of one sample
#   CPU=<last CPU>      logical CPU to pin to with taskset; CPU= disables pinning
#   OUT=bench-results/<time>  binaries, raw results (<build>.txt), env.txt,
#                     summary.txt (timings and workload) and allocs.txt
#
# For trustworthy numbers: close other programs, keep the machine on AC power,
# and treat changes smaller than the printed ± ranges as noise.
set -euo pipefail

cd "$(dirname "$0")/.."

rounds=${ROUNDS:-10}
bench=${BENCH:-.}
benchtime=${BENCHTIME:-1s}
cpu=${CPU-$(($(getconf _NPROCESSORS_ONLN) - 1))}
out=${OUT:-bench-results/$(date +%Y%m%d-%H%M%S)}
# Pinned so results don't depend on whatever benchstat is newest.
benchstat=golang.org/x/perf/cmd/benchstat@v0.0.0-20260908200009-22c9c6c9d4da

log() { printf '%s\n' "$*" >&2; }

mkdir -p "$out/bin"
out=$(cd "$out" && pwd)

builds=(default scalar simd) # column order; default is the baseline
log "building ${builds[*]}"
go test -c -o "$out/bin/default.test" ./cmd/renderer
go test -c -tags simd -o "$out/bin/scalar.test" ./cmd/renderer
GOEXPERIMENT=simd go test -c -tags simd -o "$out/bin/simd.test" ./cmd/renderer

pin=()
if [[ -n $cpu ]]; then
	if command -v taskset >/dev/null; then
		pin=(taskset -c "$cpu")
	else
		log "warning: taskset not found, not pinning to a CPU"
		cpu=
	fi
fi

governor=$(cat "/sys/devices/system/cpu/cpu${cpu:-0}/cpufreq/scaling_governor" 2>/dev/null || echo unknown)
load=$(cut -d' ' -f1 /proc/loadavg 2>/dev/null || echo 0)
{
	echo "date:       $(date -Iseconds)"
	echo "tree:       $(git describe --always --dirty) ($(git rev-parse --abbrev-ref HEAD))"
	echo "go:         $(go version)"
	echo "cpu:        $(grep -m1 'model name' /proc/cpuinfo 2>/dev/null | cut -d: -f2- | sed 's/^ //' || echo unknown)"
	echo "pinned to:  ${cpu:-no CPU}"
	echo "governor:   $governor"
	echo "load avg:   $load at start"
	echo "settings:   ROUNDS=$rounds BENCH=$bench BENCHTIME=$benchtime"
} >"$out/env.txt"
if awk -v l="$load" 'BEGIN { exit !(l > 1) }'; then
	log "warning: load average is $load; other work on this machine will add noise"
fi

# TestMain changes to ../.. to find assets/, so run from cmd/renderer.
cd cmd/renderer
for ((round = 1; round <= rounds; round++)); do
	order=("${builds[@]}")
	if ((round % 2 == 0)); then
		for ((i = ${#builds[@]} - 1, j = 0; i >= 0; i--, j++)); do
			order[j]=${builds[i]}
		done
	fi
	for b in "${order[@]}"; do
		log "round $round/$rounds: $b"
		# ebiten logs a harmless X11 notice (XGB: ...) on start-up; drop it.
		"${pin[@]}" "$out/bin/$b.test" -test.run '^$' -test.bench "$bench" \
			-test.benchtime "$benchtime" -test.count 1 -test.timeout 0 >>"$out/$b.txt" \
			2> >(grep -v '^XGB: ' >&2)
	done
done

inputs=() raw=()
for b in "${builds[@]}"; do
	inputs+=("$b=$out/$b.txt")
	raw+=("$out/$b.txt")
done
allocs='-.unit:(B/op OR allocs/op)'
if ! go run "$benchstat" -filter "$allocs" "${inputs[@]}" >"$out/summary.txt" ||
	! go run "$benchstat" -filter "${allocs#-}" "${inputs[@]}" >"$out/allocs.txt"; then
	log "benchstat failed; raw results are in $out"
	exit 1
fi
# The allocation tables are all zeros when nothing allocates, so only mention them.
if awk '{ for (i = 2; i <= NF; i++) if ($i == "allocs/op" && $(i - 1) > 0) found = 1 }
	END { exit !found }' "${raw[@]}"; then
	echo "Some benchmarks allocate; see allocs.txt." >>"$out/summary.txt"
else
	echo "No build allocates in any benchmark (allocs.txt)." >>"$out/summary.txt"
fi
cat "$out/env.txt"
echo
cat "$out/summary.txt"
log "results in $out"
