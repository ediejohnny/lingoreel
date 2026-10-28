# Speech packs: how they are made

LingoReel downloads, only when asked (Settings › Study › Voice languages), a speech pack per language: ONNX Runtime's
DLL, shared by all of them, and the language's speech model. This page tells how each file was made from its sources,
so anyone can make them again and check them against the SHA-256 the app requires
(`internal/domain/speech/pack.go`).

## ONNX Runtime 1.30.0, without telemetry

Microsoft's prebuilt Windows DLL sends telemetry through Windows' event tracing. LingoReel publishes its own build, with
the telemetry left out at compile time, and also calls `DisableTelemetryEvents` at run time, whatever DLL it loads.

- Source: <https://github.com/microsoft/onnxruntime>, tag `v1.30.0`. Its third-party components (Eigen, MPL-2.0, among
  them) are fetched by its build at the versions and SHA-1 listed in `cmake/deps.txt` of that tag; their licenses are
  in its `ThirdPartyNotices.txt`, which the app carries.
- Tools: Visual Studio 2022 Build Tools with C++ and its CMake, Python 3.10.
- Build, from the source folder:

  ```
  python tools\ci_build\build.py --build_dir <build folder> --config Release --build_shared_lib --no_telemetry
         --skip_tests --skip_submodule_sync --compile_no_warning_as_error --parallel 6
         --cmake_generator "Visual Studio 17 2022"
  ```

  `--no_telemetry` sets `onnxruntime_USE_TELEMETRY=OFF`. The resulting `onnxruntime.dll` does not contain the GUID of
  Microsoft's telemetry provider group (`4f50731a-89cf-4782-b3e0-dce8c90476ba`), which the official DLL does.
- Published as `onnxruntime-1.30.0.dll`: 17,573,888 bytes, SHA-256
  `f0e85685f946015f33a800ad4bdb7b50d10445eba1d3aabd5d4393ed60ddd213`.
- License: MIT (`internal/infrastructure/speechpack/licenses/onnxruntime-LICENSE.txt`), with its third-party notices
  (`onnxruntime-ThirdPartyNotices.txt`). Both are written beside the DLL when it is downloaded.

## The speech models

Each is a published CTC speech model (wav2vec2), Apache-2.0, at a fixed revision. LingoReel changes only its format and
precision; the weights are otherwise their authors'.

| language | source (Hugging Face) | revision |
|---|---|---|
| English | `facebook/wav2vec2-base-960h` | `22aad52d435eb6dbaf354bdad9b0da84ce7d6156` |
| Portuguese | `jonatasgrosman/wav2vec2-large-xlsr-53-portuguese` | `634ac655299bcdc46c83bc01da9bab52d2987e4f` |
| Spanish | `jonatasgrosman/wav2vec2-large-xlsr-53-spanish` | `96d7e9b4e4a78af515a3c6d3cee7c0826045d276` |
| French | `jonatasgrosman/wav2vec2-large-xlsr-53-french` | `7c79e105a6525d38e1e69f640b974b4a679723cc` |
| German | `jonatasgrosman/wav2vec2-large-xlsr-53-german` | `4b8a02957378d0f2da2ef74091156b032c485a89` |
| Italian | `jonatasgrosman/wav2vec2-large-xlsr-53-italian` | `dab04a3e00d8326052f3fb22a6ff276b822f6131` |

How each was converted (Python with `torch`, `transformers`, `safetensors`, `onnxruntime`):

1. Weights that come only as a pickle (`pytorch_model.bin`) are loaded with `torch.load(..., weights_only=True)`, which
   refuses to run code, and saved as safetensors; the model is then only ever loaded from safetensors.
2. `Wav2Vec2ForCTC` is exported with `torch.onnx.export` at opset 17: input `input_values` (batch, samples) of 16 kHz
   mono sound, output `logits` (batch, frames, alphabet), both axes dynamic.
3. The exported model is quantized with `onnxruntime.quantization.quantize_dynamic`, `op_types_to_quantize=["MatMul"]`,
   `weight_type=QuantType.QInt8`: int8 weights on the MatMul nodes only.
4. `vocab.json` is the tokenizer's vocabulary; `meta.json` names the source and revision, the sampling rate, whether the
   sound is normalized, and the blank and word-delimiter tokens. Both are carried inside the app
   (`internal/infrastructure/speechpack/packs/<language>/`) and written beside the model.

| published file | bytes | SHA-256 |
|---|---|---|
| `speech-en.onnx` | 121,923,222 | `efe5c6faa894803c1328cafdcf98be2ea1bb2b4d9026ee88afa1e42d4fa83e52` |
| `speech-pt.onnx` | 355,018,566 | `d6c29e16f1d1c53fb5514b55710c74dc35b550de2d5fbb25477b2bdfc6fbc143` |
| `speech-es.onnx` | 355,013,426 | `9e35f8db763c2ca1661004373974ce58f6731a36a350c9bf8a754b946bb587a8` |
| `speech-fr.onnx` | 355,031,930 | `d201660feed3305172b087d0147046d19b23c8b3b7492a7b85978777917ae7ec` |
| `speech-de.onnx` | 355,010,342 | `1d2cc147ec2a395d2c23c3ea13f8c4a1a1027fb6946ee228c99b2cb2e2036b6b` |
| `speech-it.onnx` | 355,016,510 | `f39959425a0d414e58252bbe5426f0293f9a61388343aba777b8c5e5ed2be644` |

- License: Apache-2.0, with the authors, revisions and changes in
  `internal/infrastructure/speechpack/licenses/models-LICENSE.txt`, written beside each model when it is downloaded.
- Training data: LibriSpeech (CC BY 4.0) for English; Common Voice (CC0 1.0) for the others.
