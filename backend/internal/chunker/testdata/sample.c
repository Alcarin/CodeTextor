#include <stdio.h>

#define MAX(a,b) ((a) > (b) ? (a) : (b))

struct Point {
    int x;
    int y;
};

void printPoint(struct Point p) {
    printf("Point: %d, %d\n", p.x, p.y);
}

int main() {
    struct Point p = {10, 20};
    printPoint(p);
    // TODO: add more shapes
    return 0;
}
