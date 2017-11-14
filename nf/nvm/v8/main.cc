// Copyright (C) 2017 go-nebulas authors
//
// This file is part of the go-nebulas library.
//
// the go-nebulas library is free software: you can redistribute it and/or
// modify it under the terms of the GNU General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// the go-nebulas library is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU General Public License for more details.
//
// You should have received a copy of the GNU General Public License
// along with the go-nebulas library.  If not, see
// <http://www.gnu.org/licenses/>.
//

#include "engine.h"
#include "lib/log_callback.h"
#include "lib/memory_storage.h"
#include "lib/blockchain.h"

#include <thread>
#include <vector>

#include <stdio.h>
#include <stdlib.h>
#include <string.h>

char *GetBlockByHash(void *handler, const char *hash) {
  char *ret = NULL;
  //TODO:not impl
  return ret;
}
char *GetTxByHash(void *handler, const char *hash) {
  char *ret = NULL;
  //TODO:not impl
  return ret;
}
char *GetAccountState(void *handler, const char *address) {
  char *ret = NULL;
  //TODO:not impl
  return ret;
}
int Send(void *handler, const char *to, const char *value) {
  //TODO:not impl
  return 0;
}

void logFunc(int level, const char *msg) {
  std::thread::id tid = std::this_thread::get_id();
  std::hash<std::thread::id> hasher;

  FILE *f = stdout;
  if (level >= LogLevel::ERROR) {
    f = stderr;
  }
  fprintf(f, "[tid-%020zu] [%s] %s\n", hasher(tid), GetLogLevelText(level),
          msg);
}

void help(const char *name) {
  printf("%s [-c <concurrency>] <Javascript File>\n", name);
  printf("%s -t <Javascript File>\n", name);
  printf("\t inject tracer code into file.\n");
  exit(1);
}

void readSource(const char *filename, char **data, size_t *size) {
  FILE *f = fopen(filename, "r");
  if (f == NULL) {
    fprintf(stderr, "file %s does not found.\n", filename);
    exit(1);
    return;
  }

  // get file size.
  fseek(f, 0L, SEEK_END);
  size_t fileSize = ftell(f);
  rewind(f);

  *size = fileSize + 1;
  *data = (char *)malloc(*size);

  size_t len = 0;
  size_t idx = 0;
  while ((len = fread(*data + idx, sizeof(char), *size - idx, f)) > 0) {
    idx += len;
    if (*size - idx <= 1) {
      *size *= 1.5;
      *data = (char *)realloc(*data, *size);
    }
  }
  *(*data + idx) = '\0';

  if (feof(f) == 0) {
    fprintf(stderr, "read file %s error.\n", filename);
    exit(1);
  }

  fclose(f);
}

void run(const char *data) {
  void *lcsHandler = CreateStorageHandler();
  void *gcsHandler = CreateStorageHandler();

  V8Engine *e = CreateEngine();
  RunScriptSource(e, data, (uintptr_t)lcsHandler, (uintptr_t)gcsHandler);
  DeleteEngine(e);

  DeleteStorageHandler(lcsHandler);
  DeleteStorageHandler(gcsHandler);
}

int main(int argc, const char *argv[]) {
  if (argc < 2) {
    help(argv[0]);
  }

  Initialize();
  InitializeLogger(logFunc);
  InitializeStorage(StorageGet, StoragePut, StorageDel);
  InitializeBlockchain(GetBlockByHash, GetTxByHash, GetAccountState, Send);

  if (strcmp(argv[1], "-c") == 0) {
    if (argc < 4) {
      help(argv[0]);
    }

    int concurrency = 1;
    int argcIdx = 1;

    concurrency = atoi(argv[2]);
    if (concurrency <= 0) {
      fprintf(stderr, "concurrency can't less than 0, set to 1.\n");
      concurrency = 1;
    }
    argcIdx += 2;

    const char *filename = argv[argcIdx];
    char *data = NULL;
    size_t size = 0;
    readSource(filename, &data, &size);

    std::vector<std::thread *> threads;
    for (int i = 0; i < concurrency; i++) {
      std::thread *thread = new std::thread(run, data);
      threads.push_back(thread);
    }

    for (int i = 0; i < concurrency; i++) {
      threads[i]->join();
    }
    free(data);

  } else if (strcmp(argv[1], "-t") == 0) {
    // inject tracer.
    if (argc < 3) {
      help(argv[0]);
    }

    const char *filename = argv[2];
    char *data = NULL;
    size_t size = 0;
    readSource(filename, &data, &size);

    void *lcsHandler = CreateStorageHandler();
    void *gcsHandler = CreateStorageHandler();

    V8Engine *e = CreateEngine();
    char *traceableSource = InjectTracingInstructions(e, data);
    if (traceableSource == NULL) {
      printf("Error.\n");
    } else {
      printf("%s\n", traceableSource);
      free(traceableSource);
    }

    DeleteEngine(e);

    DeleteStorageHandler(lcsHandler);
    DeleteStorageHandler(gcsHandler);

    free(data);
  }

  Dispose();
  return 0;
}
