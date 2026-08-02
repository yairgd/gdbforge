/*
 * Deep stack demo for GDB / gdbforge (same program as the screencast).
 *
 * Build (host / PC):
 *   gcc -g -O0 -o stack_demo examples/stack_demo.c
 *   ./bin/gdbforge ./stack_demo
 *
 * Build (ARM / MCU, as in the video — uses WFI at the end):
 *   <cross>-gcc -g -O0 -o stack_demo examples/stack_demo.c
 *
 * Try: break leaf, run, then :b callstack — or n/s/c from Code.
 */

#include <stdio.h>
#include <stdlib.h>

#if !(defined(__arm__) || defined(__aarch64__))
#include <unistd.h>
#endif

/* Level 5 — good breakpoint target */
static int leaf(int n, const char *tag)
{
	int local = n * 2;
	printf("leaf: n=%d tag=%s local=%d\n", n, tag, local);
	return local + 1;
}

/* Level 4 */
static int util(int x)
{
	int scaled = x + 10;
	printf("util: x=%d scaled=%d\n", x, scaled);
	return leaf(scaled, "from-util");
}

/* Level 3 */
static int compute(int a, int b)
{
	int sum = a + b;
	printf("compute: a=%d b=%d sum=%d\n", a, b, sum);
	return util(sum);
}

/* Level 2 */
static int process(int value)
{
	int doubled = value * 2;
	printf("process: value=%d doubled=%d\n", value, doubled);
	return compute(doubled, 3);
}

/* Level 1 — adds a few more frames via recursion */
static int descend(int depth, int acc)
{
	printf("descend: depth=%d acc=%d\n", depth, acc);
	if (depth <= 0)
		return process(acc);
	return descend(depth - 1, acc + depth);
}

static int setup(int seed)
{
	int base = seed + 1;
	printf("setup: seed=%d base=%d\n", seed, base);
	return descend(3, base);
}

int main(int argc, char **argv)
{
	int seed = 7;
	if (argc > 1)
		seed = atoi(argv[1]);

	printf("main: seed=%d\n", seed);
	int result = setup(seed);
	printf("main: result=%d\n", result);

	/* Movie / MCU: wait forever in WFI. Host build: sleep so the session stays up. */
	for (;;) {
#if defined(__arm__) || defined(__aarch64__)
		__asm volatile("wfi");
#else
		sleep(3600);
#endif
	}
	return 0;
}
