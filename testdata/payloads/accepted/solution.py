# accepted: Sum of two numbers
import sys

def main():
    line = sys.stdin.read().strip()
    if not line:
        return
    parts = line.split()
    if len(parts) >= 2:
        a, b = int(parts[0]), int(parts[1])
        print(a + b)

if __name__ == "__main__":
    main()
