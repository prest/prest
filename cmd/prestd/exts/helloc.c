#include <stdio.h>
#include "helloc.h"

void Hello() {
    printf("Hello world!\n");
}

// gcc helloc.c  -fPIC -shared -o lib/helloc.so
