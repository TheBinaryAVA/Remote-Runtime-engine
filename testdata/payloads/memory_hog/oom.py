# memory_hog: tests Memory Limit Exceeded (MLE / OOM)
chunks = []
try:
    while True:
        # Allocate 10MB chunks rapidly until cgroup kills process
        chunks.append(bytearray(10 * 1024 * 1024))
except MemoryError:
    pass
