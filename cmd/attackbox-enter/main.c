#define _GNU_SOURCE
#include <sys/prctl.h>
#include <sys/syscall.h>
#include <unistd.h>
#include <stdint.h>
#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <pwd.h>
#include <grp.h>

#define _LINUX_CAPABILITY_VERSION_3 0x20080522
#define PR_CAPBSET_DROP 24

// Target capabilities to permanently strip from container processes
#define CAP_SYS_MODULE 16
#define CAP_SYS_RAWIO  17
#define CAP_SYS_BOOT   22
#define CAP_MKNOD      27

struct cap_header {
    uint32_t version;
    int pid;
};

struct cap_data {
    uint32_t effective;
    uint32_t permitted;
    uint32_t inheritable;
};

int main(int argc, char **argv) {
    struct cap_header hdr;
    hdr.version = _LINUX_CAPABILITY_VERSION_3;
    hdr.pid = 0; // target self

    struct cap_data data[2];
    data[0].effective = 0;
    data[0].permitted = 0;
    data[0].inheritable = 0;
    data[1].effective = 0;
    data[1].permitted = 0;
    data[1].inheritable = 0;

    // 1. Get current capability sets
    if (syscall(SYS_capget, &hdr, data) == 0) {
        // Strip CAP_SYS_MODULE (16), CAP_SYS_RAWIO (17), CAP_SYS_BOOT (22), CAP_MKNOD (27)
        uint32_t mask0 = (1U << CAP_SYS_MODULE) | (1U << CAP_SYS_RAWIO) | (1U << CAP_SYS_BOOT) | (1U << CAP_MKNOD);
        data[0].effective &= ~mask0;
        data[0].permitted &= ~mask0;
        data[0].inheritable &= ~mask0;

        // 2. capset: explicitly remove from effective, permitted, and inheritable sets
        if (syscall(SYS_capset, &hdr, data) != 0) {
            perror("capset failed");
            return 1;
        }
    } else {
        perror("capget failed");
        return 1;
    }

    // 3. Drop from Capability Bounding Set (PR_CAPBSET_DROP)
    // Even if user escalates via sudo, bounding set limits prevent acquiring these caps.
    if (prctl(PR_CAPBSET_DROP, CAP_SYS_BOOT, 0, 0, 0) != 0) {
        perror("drop CAP_SYS_BOOT failed");
    }
    if (prctl(PR_CAPBSET_DROP, CAP_SYS_RAWIO, 0, 0, 0) != 0) {
        perror("drop CAP_SYS_RAWIO failed");
    }
    if (prctl(PR_CAPBSET_DROP, CAP_SYS_MODULE, 0, 0, 0) != 0) {
        perror("drop CAP_SYS_MODULE failed");
    }
    if (prctl(PR_CAPBSET_DROP, CAP_MKNOD, 0, 0, 0) != 0) {
        perror("drop CAP_MKNOD failed");
    }

    // Note: PR_SET_NO_NEW_PRIVS is deliberately NOT set here so that
    // the standard user in the sudo group can execute passwordless sudo.
    // The dropped Bounding Set guarantees that elevated root still cannot
    // reboot the host or insert kernel modules.

    // 4. Parse user target
    const char *target_user = "box";
    int arg_idx = 1;
    if (arg_idx < argc && strcmp(argv[arg_idx], "--root") == 0) {
        target_user = NULL;
        arg_idx++;
    } else if (arg_idx + 1 < argc && strcmp(argv[arg_idx], "--user") == 0) {
        target_user = argv[arg_idx + 1];
        arg_idx += 2;
    }

    // 5. Drop privileges to non-root user if specified
    if (target_user != NULL) {
        struct passwd *pw = getpwnam(target_user);
        uid_t uid = 1001;
        gid_t gid = 1013;
        const char *homedir = "/home/box";
        const char *shell = "/bin/bash";

        if (pw != NULL) {
            uid = pw->pw_uid;
            gid = pw->pw_gid;
            homedir = pw->pw_dir;
            if (pw->pw_shell && pw->pw_shell[0] != '\0') {
                shell = pw->pw_shell;
            }
            initgroups(pw->pw_name, pw->pw_gid);
        } else {
            gid_t groups[2] = {gid, 27};
            setgroups(2, groups);
        }

        if (setgid(gid) != 0) {
            perror("setgid failed");
            return 1;
        }
        if (setuid(uid) != 0) {
            perror("setuid failed");
            return 1;
        }
        setenv("USER", target_user, 1);
        setenv("LOGNAME", target_user, 1);
        setenv("HOME", homedir, 1);
        setenv("SHELL", shell, 1);
        if (chdir(homedir) != 0) {
            perror("chdir home failed");
        }
    }

    // 6. Exec target shell / command
    char *default_argv[] = {"/bin/bash", "-l", NULL};
    char **target_argv = (arg_idx < argc) ? &argv[arg_idx] : default_argv;

    execvp(target_argv[0], target_argv);
    perror("execvp target command failed");
    return 1;
}
