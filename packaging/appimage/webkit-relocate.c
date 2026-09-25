#define _GNU_SOURCE
#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <unistd.h>
#include <dlfcn.h>
#include <spawn.h>
#include <limits.h>

#define TARGET_PREFIX "/usr/lib/x86_64-linux-gnu/webkitgtk-6.0/"

static const char *relocate_path(const char *path, char *buffer, size_t buffer_size) {
    if (!path) {
        return NULL;
    }
    if (strncmp(path, TARGET_PREFIX, strlen(TARGET_PREFIX)) == 0) {
        const char *appdir = getenv("APPDIR");
        if (appdir && *appdir) {
            snprintf(buffer, buffer_size, "%s%s", appdir, path);
            return buffer;
        }
    }
    return path;
}

typedef int (*orig_execve_t)(const char *, char *const [], char *const []);
typedef int (*orig_execv_t)(const char *, char *const []);
typedef int (*orig_execvp_t)(const char *, char *const []);
typedef int (*orig_execvpe_t)(const char *, char *const [], char *const []);
typedef int (*orig_posix_spawn_t)(pid_t *, const char *, const posix_spawn_file_actions_t *,
                                  const posix_spawnattr_t *, char *const [], char *const []);
typedef int (*orig_posix_spawnp_t)(pid_t *, const char *, const posix_spawn_file_actions_t *,
                                   const posix_spawnattr_t *, char *const [], char *const []);

int execve(const char *path, char *const argv[], char *const envp[]) {
    static orig_execve_t orig_execve = NULL;
    if (!orig_execve) {
        orig_execve = (orig_execve_t)dlsym(RTLD_NEXT, "execve");
    }
    char buffer[PATH_MAX];
    const char *new_path = relocate_path(path, buffer, sizeof(buffer));
    return orig_execve(new_path, argv, envp);
}

int execv(const char *path, char *const argv[]) {
    static orig_execv_t orig_execv = NULL;
    if (!orig_execv) {
        orig_execv = (orig_execv_t)dlsym(RTLD_NEXT, "execv");
    }
    char buffer[PATH_MAX];
    const char *new_path = relocate_path(path, buffer, sizeof(buffer));
    return orig_execv(new_path, argv);
}

int execvp(const char *file, char *const argv[]) {
    static orig_execvp_t orig_execvp = NULL;
    if (!orig_execvp) {
        orig_execvp = (orig_execvp_t)dlsym(RTLD_NEXT, "execvp");
    }
    char buffer[PATH_MAX];
    const char *new_path = relocate_path(file, buffer, sizeof(buffer));
    return orig_execvp(new_path, argv);
}

int execvpe(const char *file, char *const argv[], char *const envp[]) {
    static orig_execvpe_t orig_execvpe = NULL;
    if (!orig_execvpe) {
        orig_execvpe = (orig_execvpe_t)dlsym(RTLD_NEXT, "execvpe");
    }
    char buffer[PATH_MAX];
    const char *new_path = relocate_path(file, buffer, sizeof(buffer));
    return orig_execvpe(new_path, argv, envp);
}

int posix_spawn(pid_t *pid, const char *path, const posix_spawn_file_actions_t *file_actions,
                const posix_spawnattr_t *attrp, char *const argv[], char *const envp[]) {
    static orig_posix_spawn_t orig_posix_spawn = NULL;
    if (!orig_posix_spawn) {
        orig_posix_spawn = (orig_posix_spawn_t)dlsym(RTLD_NEXT, "posix_spawn");
    }
    char buffer[PATH_MAX];
    const char *new_path = relocate_path(path, buffer, sizeof(buffer));
    return orig_posix_spawn(pid, new_path, file_actions, attrp, argv, envp);
}

int posix_spawnp(pid_t *pid, const char *file, const posix_spawn_file_actions_t *file_actions,
                 const posix_spawnattr_t *attrp, char *const argv[], char *const envp[]) {
    static orig_posix_spawnp_t orig_posix_spawnp = NULL;
    if (!orig_posix_spawnp) {
        orig_posix_spawnp = (orig_posix_spawnp_t)dlsym(RTLD_NEXT, "posix_spawnp");
    }
    char buffer[PATH_MAX];
    const char *new_path = relocate_path(file, buffer, sizeof(buffer));
    return orig_posix_spawnp(pid, new_path, file_actions, attrp, argv, envp);
}
