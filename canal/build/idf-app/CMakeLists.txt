cmake_minimum_required(VERSION 3.16)

set(IDF_TARGET esp32s3)

include($ENV{IDF_PATH}/tools/cmake/project.cmake)
project(canal_idf_bridge)

# TinyGo runtime expects legacy bss boundary symbols.
idf_build_get_property(PROJECT_EXECUTABLE EXECUTABLE)
target_link_options(${PROJECT_EXECUTABLE} PRIVATE
	"-Wl,--defsym=_sbss=_bss_start"
	"-Wl,--defsym=_ebss=_bss_end"
	"-Wl,-u,usb_serial_jtag_vfs_use_nonblocking"
	"-Wl,-u,usb_serial_jtag_driver_install"
	"-Wl,-u,usb_serial_jtag_vfs_use_driver"
	"-Wl,-u,usb_serial_jtag_is_driver_installed"
)
