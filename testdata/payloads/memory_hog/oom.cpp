// memory_hog: tests Memory Limit Exceeded (MLE / OOM)
#include <iostream>
#include <vector>
#include <cstring>

int main() {
    std::vector<char*> allocations;
    while (true) {
        // Allocate 10MB blocks and touch pages to ensure physical resident memory allocation
        char* block = new char[10 * 1024 * 1024];
        std::memset(block, 0xAA, 10 * 1024 * 1024);
        allocations.push_back(block);
    }
    return 0;
}
