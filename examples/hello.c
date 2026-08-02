#include <stdio.h>
#include <unistd.h>


int main(void) {

  char b[100];
  int i=0;
  while (1) { 

    snprintf(b, sizeof(b),"hello, gdbforge %d",i++);
    printf("%s\n",b);
//    usleep(100);

  }
  return 0;
}
