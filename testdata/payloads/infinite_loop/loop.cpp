// infinite_loop: tests Time Limit Exceeded (TLE)
#include <iostream>
#include <chrono>
#include <thread>

int main() {
    while (true) {
        std::this_thread::sleep_for(std::chrono::milliseconds(10));
    }
    return 0;
}
