/*
 * SPDX-License-Identifier: Apache-2.0
 *
 * Retire any interrupt a warm restart left active.
 *
 * Restarting by jumping to __start with no system reset in between - which is
 * exactly what a debugger "load" followed by "set $pc = 0x0" does - can land
 * between an ISR's GICC_IAR read and its GICC_EOIR write. The CPU interface
 * keeps that interrupt's priority as the running priority, and because
 * gic_dist_init() gives every SPI priority 0, nothing is ever signalled again:
 * the timer keeps counting, its interrupt keeps going pending, and no tick is
 * ever delivered, so k_sleep() never returns.
 *
 * GICv1 has no GICD_ICACTIVERn - only the read-only active bits at 0x300 - so
 * gic_dist_init() cannot clear the state and it survives into the application,
 * where it can be read and retired. v1 has no split EOI either, so one
 * GICC_EOIR write both pops a level off the priority stack and deactivates the
 * interrupt. On a cold boot nothing reads back active and this is a no-op.
 *
 * PRE_KERNEL_1 is both late and early enough. The GIC is set up in
 * z_arm_interrupt_init(), called from z_prep_c() before z_cstart(), so the CPU
 * interface already accepts GICC_EOIR writes by the time any SYS_INIT entry
 * runs; the system timer registers at PRE_KERNEL_2, so it has not been
 * programmed yet. Interrupts are masked at the core for all of PRE_KERNEL_1, so
 * none can be taken while the priority stack unwinds.
 */

#include <zephyr/kernel.h>
#include <zephyr/init.h>

#ifdef CONFIG_GIC_V1

#include <zephyr/drivers/interrupt_controller/gic.h>
#include <zephyr/sys/sys_io.h>

/* gic.h only declares the registers the normal v1/v2 paths touch. */
#define GICC_RPR      (GIC_CPU_BASE + 0x14)
#define GICC_RPR_IDLE 0xff

static int gic_drain_active(void)
{
	unsigned int gic_irqs, i, bit;

	gic_irqs = ((sys_read32(GICD_TYPER) & GICD_TYPER_ITLINESNUM_MASK) + 1) * 32;
	if (gic_irqs > 1020) {
		gic_irqs = 1020;
	}

	for (i = 0; i < gic_irqs; i += 32) {
		uint32_t active = sys_read32(GICD_ISACTIVERn + i / 8);

		for (bit = 0; bit < 32; bit++) {
			if ((active & BIT(bit)) != 0U) {
				sys_write32(i + bit, GICC_EOIR);
			}
		}
	}

	__ASSERT(sys_read32(GICC_RPR) == GICC_RPR_IDLE,
		 "GIC CPU interface still busy at priority 0x%x; interrupts "
		 "will never be signalled", sys_read32(GICC_RPR));

	return 0;
}

SYS_INIT(gic_drain_active, PRE_KERNEL_1, 0);

#endif /* CONFIG_GIC_V1 */
