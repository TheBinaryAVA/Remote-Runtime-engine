# fork_bomb: tests pids.max limit and process creation exhaustion
import os
import sys

def bomb():
    try:
        while True:
            os.fork()
    except Exception as e:
        sys.stderr.write(f"Fork failed safely: {e}\n")
        sys.exit(1)

if __name__ == "__main__":
    bomb()
