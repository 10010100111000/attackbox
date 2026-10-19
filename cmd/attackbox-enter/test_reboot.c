#include <unistd.h>
#include <sys/syscall.h>
#include <stdio.h>
#include <errno.h>

int main(int argc, char **argv) {
    if (argc > 1 && argv[1][0] == 'm') {
        // Direct kernel syscall to test CAP_SYS_MODULE rejection
        long ret = syscall(SYS_finit_module, -1, "", 0);
        printf("finit_module syscall ret=%ld, errno=%d (EPERM=%d)\n", ret, errno, EPERM);
        if (ret == -1 && errno == EPERM) {
            printf("KERNEL_MODULE_EPERM_CONFIRMED\n");
            return 0;
        }
        return 1;
    }

    // Direct kernel syscall to test CAP_SYS_BOOT rejection
    long ret = syscall(SYS_reboot, 0xfee1dead, 0x28121969, 0x01234567, 0);
    printf("reboot syscall ret=%ld, errno=%d (EPERM=%d)\n", ret, errno, EPERM);
    if (ret == -1 && errno == EPERM) {
        printf("KERNEL_EPERM_CONFIRMED\n");
        return 0;
    }
    return 1;
}
