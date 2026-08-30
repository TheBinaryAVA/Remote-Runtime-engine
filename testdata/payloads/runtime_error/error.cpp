// runtime_error: Division by zero / SIGFPE
#include <iostream>

int main() {
    volatile int a = 42;
    volatile int b = 0;
    volatile int c = a / b;
    std::cout << c << "\n";
    return 0;
}
