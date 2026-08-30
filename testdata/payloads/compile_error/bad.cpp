// compile_error: Invalid syntax missing semicolon and unknown types
#include <iostream>

int main() {
    ThisIsAnInvalidType x = 123
    std::cout << x << std::endl
    return 0;
}
