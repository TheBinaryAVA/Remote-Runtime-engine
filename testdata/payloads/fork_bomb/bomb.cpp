// fork_bomb: tests pids.max limit and process creation exhaustion
#include <unistd.h>
#include <iostream>

int main() {
    while (true) {
        pid_t p = fork();
        if (p < 0) {
            std::cerr << "Fork failed due to pids limit\n";
            return 1;
        }
    }
    return 0;
}
