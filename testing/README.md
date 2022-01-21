# Pstack
For testing purposes on non CentOS/RHEL the pstack which works
by invoking gdb isn't available.

The binary is often more flaky (can't find the __DYNAMIC section randomly)
so we provide the gdb wrapper here for testing purposes.

This came from the gdb-7.6.1-120.el7.x86_64 RPM on CentOS7
