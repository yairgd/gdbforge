/*
 * SPDX-License-Identifier: Apache-2.0
 */

#ifndef DEMO_H
#define DEMO_H

#include <zephyr/kernel.h>
#include <stdint.h>

#define DEMO_WORKERS 2

/*
 * Shared state, written by main() and by both workers. Deliberately a plain
 * struct rather than atomics: the point is to have something a watchpoint can
 * sit on, with a mutex around it so stepping through one writer while another
 * is blocked is observable in the Threads pane.
 */
struct demo_stats {
	uint32_t iterations;
	uint32_t worker_wakeups[DEMO_WORKERS];
	int32_t last_sample;
};

extern struct demo_stats demo_stats;
extern struct k_mutex demo_lock;

void workers_start(void);

#endif /* DEMO_H */
