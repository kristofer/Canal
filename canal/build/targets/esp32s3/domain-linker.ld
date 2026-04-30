/* Canal domain linker script for ESP32-S3
 *
 * Each domain binary is a self-contained ELF flashed to its own partition.
 * The kernel's ELF loader reads program headers, copies LOAD segments into
 * the domain's RAM region, and jumps to domain_entry.
 *
 * Flash XIP mapping: physical 0xN0000 → virtual 0x420N0000
 * The DOMAIN_FLASH origin below must match the domain's partition offset
 * after adding the XIP base (0x42000000).
 *
 * Usage with TinyGo (pass via -ldflags):
 *   tinygo build -ldflags="-T build/targets/esp32s3/domain-linker.ld" ...
 *
 * If TinyGo ignores the external script, use -ldflags="-e domain_entry"
 * to at least set the ELF entry point to domain_entry.
 */

ENTRY(domain_entry)

MEMORY
{
    /* Domain code lives in its flash partition (XIP).
     * Loader sets up the correct virtual address per partition. */
    DOMAIN_FLASH (rx)  : ORIGIN = 0x42100000, LENGTH = 64K

    /* Per-domain SRAM region — kernel allocates and passes base at spawn time */
    DOMAIN_RAM   (rw)  : ORIGIN = 0x3FC80000, LENGTH = 16K
    DOMAIN_HEAP  (rw)  : ORIGIN = 0x3FC84000, LENGTH = 4K
}

SECTIONS
{
    .text : {
        KEEP(*(.text.domain_entry))   /* entry point first */
        *(.text*)
        *(.rodata*)
    } > DOMAIN_FLASH

    .data : {
        _data_start = .;
        *(.data*)
        _data_end = .;
    } > DOMAIN_RAM AT > DOMAIN_FLASH

    .bss (NOLOAD) : {
        _bss_start = .;
        *(.bss*)
        *(COMMON)
        _bss_end = .;
    } > DOMAIN_RAM

    /* Discard debug and other non-essential sections to save flash */
    /DISCARD/ : {
        *(.debug*)
        *(.comment)
        *(.eh_frame*)
    }
}
