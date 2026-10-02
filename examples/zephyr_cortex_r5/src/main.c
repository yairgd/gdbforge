/*
 * SPDX-License-Identifier: Apache-2.0
 *
 * Zephyr on Cortex-R5 debugging demo. Three threads (main plus two workers), a
 * shared struct behind a mutex, and a call chain deep enough to be worth a
 * backtrace.
 */

#include <zephyr/kernel.h>
#include <zephyr/init.h>
#include <zephyr/sys/printk.h>

#include "demo.h"

struct demo_stats demo_stats;
K_MUTEX_DEFINE(demo_lock);

#ifdef CONFIG_ARM_ON_ENTER_CPU_IDLE_HOOK
/*
 * Returning false makes arch_cpu_idle() skip the WFI and spin instead. A
 * sleeping core cannot answer the probe's memory reads: it halts the core to
 * poll the RTT buffer, gdb sees that halt as SIGTRAP and stops us. Costs idle
 * power, so this follows the hook, which follows RTT_CONSOLE — it is gone in a
 * uart0-console build.
 */
bool z_arm_on_enter_cpu_idle(void)
{
	return false;
}
#endif

/*
 * Boot progress marker, for breakpoints that have to fire before main().
 * Lives in .noinit, so 0 means "never got here". Each checkpoint shifts a
 * nibble in, which makes boot_stage read as a trail: 0x12345678.
 */
__attribute__((section(".noinit"), used))
volatile uint32_t boot_stage;

static inline void mark(uint32_t v)
{
	boot_stage = (boot_stage << 4) | (v & 0xf);
}

static int mk_pk1_first(void)  { mark(1); return 0; }
static int mk_pk1_last(void)   { mark(2); return 0; }
static int mk_pk2_first(void)  { mark(3); return 0; }
static int mk_pk2_last(void)   { mark(4); return 0; }
static int mk_post_first(void) { mark(5); return 0; }
static int mk_post_last(void)  { mark(6); return 0; }
static int mk_app_first(void)  { mark(7); return 0; }
static int mk_app_last(void)   { mark(8); return 0; }

/* Priority 0 runs first within a level, 99 last, so each pair brackets it. */
SYS_INIT(mk_pk1_first,  PRE_KERNEL_1, 0);
SYS_INIT(mk_pk1_last,   PRE_KERNEL_1, 99);
SYS_INIT(mk_pk2_first,  PRE_KERNEL_2, 0);
SYS_INIT(mk_pk2_last,   PRE_KERNEL_2, 99);
SYS_INIT(mk_post_first, POST_KERNEL, 0);
SYS_INIT(mk_post_last,  POST_KERNEL, 99);
SYS_INIT(mk_app_first,  APPLICATION, 0);
SYS_INIT(mk_app_last,   APPLICATION, 99);

/* Recursive on purpose: gives `bt` something to unwind and `finish` something
 * to return from. Keep n small, the stacks here are modest.
 */
static uint32_t fib(uint32_t n)
{
	if (n < 2U) {
		return n;
	}

	return fib(n - 1U) + fib(n - 2U);
}

static int32_t accumulate(int32_t acc, uint32_t term)
{
	return acc + (int32_t)term;
}

static int32_t compute_sample(uint32_t round)
{
	uint32_t depth = (round % 8U) + 2U;
	int32_t acc = 0;

	for (uint32_t i = 0; i < depth; i++) {
		acc = accumulate(acc, fib(i));
	}

	return acc;
}

/* Takes the lock, so stepping through it while a worker is waiting shows the
 * worker blocked in the Threads pane.
 */
static int32_t run_iteration(void)
{
	int32_t sample;

	k_mutex_lock(&demo_lock, K_FOREVER);
	demo_stats.iterations++;
	sample = compute_sample(demo_stats.iterations);
	demo_stats.last_sample = sample;
	k_mutex_unlock(&demo_lock);

	return sample;
}

int main(void)
{
	printk("zephyr cortex-r5 debug demo on %s\n", CONFIG_BOARD_TARGET);

	workers_start();

	while (1) {
		int32_t sample = run_iteration();

		printk("iteration %u sample %d\n", demo_stats.iterations, sample);
		k_sleep(K_MSEC(1000));
	}

	return 0;
}
