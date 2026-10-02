/*
 * SPDX-License-Identifier: Apache-2.0
 *
 * Two demo threads, started from main().
 */

#include <zephyr/kernel.h>
#include <zephyr/random/random.h>
#include <zephyr/sys/printk.h>

#include "demo.h"

#define WORKER_STACK_SIZE 1024
#define WORKER_PRIORITY   5

#define WORKER_DELAY_MIN_MS 1000
#define WORKER_DELAY_MAX_MS 2000

static K_THREAD_STACK_DEFINE(worker0_stack, WORKER_STACK_SIZE);
static K_THREAD_STACK_DEFINE(worker1_stack, WORKER_STACK_SIZE);

static struct k_thread worker0_data;
static struct k_thread worker1_data;

/* Marsaglia xorshift32. Never returns 0 and never reaches it from non-zero. */
static uint32_t xorshift32(uint32_t *state)
{
	uint32_t x = *state;

	x ^= x << 13;
	x ^= x >> 17;
	x ^= x << 5;
	*state = x;

	return x;
}

static void worker_entry(void *id, void *unused1, void *unused2)
{
	int slot = POINTER_TO_INT(id);
	/*
	 * Per-thread state, on this thread's own stack. Calling sys_rand32_get()
	 * in the loop instead would hand both threads alternating draws from one
	 * shared sequence, which correlates them - the whole point here is that
	 * the two delays are independent.
	 */
	uint32_t state = sys_rand32_get() | 1U;

	ARG_UNUSED(unused1);
	ARG_UNUSED(unused2);

	while (1) {
		uint32_t delay_ms = WORKER_DELAY_MIN_MS +
			xorshift32(&state) % (WORKER_DELAY_MAX_MS - WORKER_DELAY_MIN_MS + 1);

		k_mutex_lock(&demo_lock, K_FOREVER);
		demo_stats.worker_wakeups[slot]++;
		k_mutex_unlock(&demo_lock);

		printk("worker%d awake %u times (next in %u ms)\n",
		       slot, demo_stats.worker_wakeups[slot], delay_ms);
		k_sleep(K_MSEC(delay_ms));
	}
}

void workers_start(void)
{
	k_tid_t tid;

	tid = k_thread_create(&worker0_data, worker0_stack, WORKER_STACK_SIZE,
			      worker_entry, INT_TO_POINTER(0), NULL, NULL,
			      WORKER_PRIORITY, 0, K_NO_WAIT);
	/* Named, so the Threads pane shows worker0/worker1 rather than unnamed
	 * entries. Needs CONFIG_THREAD_NAME=y.
	 */
	k_thread_name_set(tid, "worker0");

	/* Staggered, so the two do not print together on the very first pass. */
	tid = k_thread_create(&worker1_data, worker1_stack, WORKER_STACK_SIZE,
			      worker_entry, INT_TO_POINTER(1), NULL, NULL,
			      WORKER_PRIORITY, 0, K_MSEC(500));
	k_thread_name_set(tid, "worker1");
}
