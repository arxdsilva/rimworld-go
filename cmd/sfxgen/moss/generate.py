"""Make one sound effect with the local MOSS-SoundEffect v2.0 model (cmd/sfxgen -backend moss).

cmd/sfxgen runs this once per take with the Python of a virtualenv that has the
moss_soundeffect_v2 package installed (see cmd/sfxgen/main.go for the setup and
the default paths). Run it with `python -I`. It writes a mono 24-bit WAV at the
model's sample rate and prints one JSON line with what happened.

    python -I generate.py --model DIR --prompt TEXT --seconds 1.0 --seed 7 --out take.wav
        [--steps 100] [--cfg 4.0] [--device auto|mps|cuda|cpu]

The model, the virtualenv and the Hugging Face cache never live in the repo.
"""
import argparse
import json
import os
import resource
import sys
import time
import warnings

# torch.compile's MPS backend cannot build the model's float64 kernels; eager is fine.
os.environ.setdefault("TORCHDYNAMO_DISABLE", "1")
os.environ.setdefault("HF_HUB_OFFLINE", "1")
warnings.filterwarnings("ignore")


def patch_for_mps():
    """MPS has no float64 or complex128: do those parts on the CPU, in place, at runtime only."""
    import torch
    from moss_soundeffect_v2.diffsynth.models import wan_audio_dit, wan_video_dit
    from moss_soundeffect_v2.diffsynth.pipelines import wan_audio

    def sinusoidal_embedding_1d(dim, position):
        pos = position.detach().float().cpu().double()
        steps = torch.arange(dim // 2, dtype=torch.float64).div(dim // 2)
        sinusoid = torch.outer(pos, torch.pow(10000, -steps))
        x = torch.cat([torch.cos(sinusoid), torch.sin(sinusoid)], dim=1)
        return x.to(device=position.device, dtype=position.dtype)

    orig_freqs = wan_audio_dit.precompute_freqs_cis

    def precompute_freqs_cis(*a, **kw):
        return orig_freqs(*a, **kw).to(torch.complex64)

    for m in (wan_audio, wan_audio_dit, wan_video_dit):
        m.sinusoidal_embedding_1d = sinusoidal_embedding_1d
    wan_audio_dit.precompute_freqs_cis = precompute_freqs_cis


def maxrss_bytes():
    rss = resource.getrusage(resource.RUSAGE_SELF).ru_maxrss
    return rss if sys.platform == "darwin" else rss * 1024  # Linux reports KiB


def pick_device(want):
    import torch
    if want != "auto":
        return want
    if torch.cuda.is_available():
        return "cuda"
    if torch.backends.mps.is_available():
        return "mps"
    return "cpu"


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("--model", required=True, help="MOSS-SoundEffect v2.0 snapshot directory (has model_index.json)")
    ap.add_argument("--prompt", required=True)
    ap.add_argument("--seconds", type=float, required=True)
    ap.add_argument("--seed", type=int, required=True)
    ap.add_argument("--out", required=True)
    ap.add_argument("--steps", type=int, default=100)
    ap.add_argument("--cfg", type=float, default=4.0)
    ap.add_argument("--device", default="auto")
    args = ap.parse_args()
    if not os.path.isfile(os.path.join(args.model, "model_index.json")):
        sys.exit(f"{args.model}: not a MOSS-SoundEffect snapshot (no model_index.json)")

    import soundfile as sf
    import torch
    from moss_soundeffect_v2 import MossSoundEffectPipeline

    device = pick_device(args.device)
    if device == "mps":
        patch_for_mps()
    t0 = time.time()
    pipe = MossSoundEffectPipeline.from_pretrained(args.model, torch_dtype=torch.bfloat16, device=device)
    load_s = time.time() - t0
    t1 = time.time()
    audio = pipe(prompt=args.prompt, seconds=args.seconds, num_inference_steps=args.steps,
                 cfg_scale=args.cfg, seed=args.seed, progress_bar_cmd=lambda x: x)
    if device == "mps":
        torch.mps.synchronize()
    a = audio.float()
    if bool(torch.isnan(a).any()):
        sys.exit("the model produced NaN samples")
    os.makedirs(os.path.dirname(os.path.abspath(args.out)), exist_ok=True)
    sf.write(args.out, a[0, 0].cpu().numpy(), pipe.sample_rate, subtype="PCM_24")
    print(json.dumps({"device": device, "load_seconds": round(load_s, 1), "gen_seconds": round(time.time() - t1, 1),
                      "sample_rate": pipe.sample_rate, "peak": round(float(a.abs().max()), 4),
                      "maxrss_gb": round(maxrss_bytes() / 1e9, 2)}), flush=True)


if __name__ == "__main__":
    main()
