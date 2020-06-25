#!/bin/bash -eux
#
# Copyright 2019 The Outline Authors
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#      http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

# Builds a tun2socks Android library for Intra or Outline.
# Usage: ./build_android.sh [intra|outline]
readonly BUILD_DIR=build/android
readonly LOG_FILE=$(mktemp)
readonly TARGET=android-${1:-outline}

rm -rf $BUILD_DIR
make clean && make $TARGET 2>&1 | tee $LOG_FILE

# Parse the Go working directory to copy the unstripped JNI binaries so symbols can be uploaded
# to crash reporting tools.
readonly GO_WORK_DIR=$(cat $LOG_FILE | grep "WORK=" | cut -f2 -d=)
if [ ! -z $GO_WORK_DIR ]; then
  echo "Copying JNI binaries from: $GO_WORK_DIR"
  # Make the Go working directory writable (it's read-only) so we can remove it when we're done.
  chmod -R +w $GO_WORK_DIR
  readonly JNI_DIR=$BUILD_DIR/jni
  mkdir -p $JNI_DIR && cp -R $GO_WORK_DIR/android/src/main/jniLibs/ $JNI_DIR/
  rm -rf $GO_WORK_DIR
fi
