"""Optional, local-only EmbeddingGemma 2 server. No inference-time downloads.

Global defaults/presets are owned by Go. This worker loads only requested
encoders; CPU uses float32, GPU uses bfloat16 when supported (never float16).
"""
import argparse
import hmac
import json
import math
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer
from pathlib import Path
import threading


def selective_config(modalities):
    active = set(modalities.split(','))
    if not active <= {'text', 'image', 'audio', 'video'} or 'text' not in active:
        raise ValueError('invalid modalities')
    config = {}
    if not active & {'image', 'video'}:
        config['vision_config'] = None
    if 'audio' not in active:
        config['audio_config'] = None
    return config


def safe_media(path, root):
    resolved = Path(path).resolve(strict=True)
    resolved.relative_to(root.resolve(strict=True))
    if not resolved.is_file() or resolved.stat().st_size > 64 * 1024 * 1024:
        raise ValueError('invalid media file')
    return str(resolved)


def prepare_input(item, root, active):
    if isinstance(item, str):
        if len(item) > 32768:
            raise ValueError('input too long')
        return item
    if not isinstance(item, dict) or set(item) - {'text', 'image', 'audio', 'video'}:
        raise ValueError('invalid input')
    result = {'text': prepare_input(item.get('text', ''), root, active)}
    for modality in ['image', 'audio', 'video']:
        media = item.get(modality)
        if not media:
            continue
        if modality not in active:
            raise ValueError('inactive modality')
        if modality == 'image':
            if not isinstance(media, list) or len(media) > 4:
                raise ValueError('image limit')
            result[modality] = [safe_media(p, root) for p in media]
        else:
            if not isinstance(media, str):
                raise ValueError('one audio/video per input')
            result[modality] = safe_media(media, root)
    return result


def materialize_video(inputs):
    """Decode with the bundled PyAV wheels; no system FFmpeg/torchcodec required.

    At most 16 frames, sampled at one frame/second from the beginning. Explicit
    bound keeps media plus a note inside the model's shared context budget.
    """
    for item in inputs:
        if isinstance(item, dict) and item.get('video'):
            import av
            import numpy as np
            frames = []
            with av.open(item['video']) as container:
                stream = container.streams.video[0]
                next_second = 0
                for frame in container.decode(stream):
                    timestamp = float(frame.time or 0)
                    if timestamp >= next_second:
                        frames.append(frame.to_ndarray(format='rgb24'))
                        next_second += 1
                        if len(frames) >= 16:
                            break
            if not frames:
                raise ValueError('empty video')
            item['video'] = np.stack(frames)
    return inputs


def configure_model(model):
    video_processor = getattr(model[0].processor, 'video_processor', None)
    if video_processor is not None:
        video_processor.do_sample_frames = False  # PyAV already sampled frames.
    return model


def make_handler(model, root, active, key, lock):
    class Handler(BaseHTTPRequestHandler):
        def log_message(self, *_):
            pass  # No snippets, tokens, media paths or secrets in request logs.

        def reply(self, status, data):
            body = json.dumps(data, allow_nan=False).encode()
            self.send_response(status)
            self.send_header('Content-Type', 'application/json')
            self.send_header('Content-Length', str(len(body)))
            self.end_headers()
            self.wfile.write(body)

        def authorized(self):
            return hmac.compare_digest(self.headers.get('Authorization', ''), 'Bearer ' + key)

        def do_GET(self):
            if not self.authorized():
                self.reply(401, {'error': 'unauthorized'})
            elif self.path == '/health':
                self.reply(200, {'status': 'ok', 'modalities': sorted(active)})
            else:
                self.reply(404, {'error': 'not found'})

        def do_POST(self):
            if not self.authorized():
                self.reply(401, {'error': 'unauthorized'})
                return
            if self.path != '/v1/embeddings':
                self.reply(404, {'error': 'not found'})
                return
            try:
                length = int(self.headers.get('Content-Length', '0'))
                if not 0 < length <= 131072:
                    raise ValueError('request size')
                inputs = json.loads(self.rfile.read(length)).get('input')
                if not isinstance(inputs, list) or not 0 < len(inputs) <= 8:
                    raise ValueError('input count')
                inputs = [prepare_input(item, root, active) for item in inputs]
                inputs = materialize_video(inputs)
                with lock:
                    vectors = model.encode(inputs, normalize_embeddings=True,
                                           show_progress_bar=False, convert_to_numpy=True)
                rows = vectors.tolist()
                if len(rows) != len(inputs) or any(len(row) != 768 or not all(math.isfinite(x) for x in row) for row in rows):
                    raise ValueError('invalid model output')
                self.reply(200, {'data': [{'index': i, 'embedding': row} for i, row in enumerate(rows)]})
            except Exception as exc:
                # Keep source contents and detailed model errors out of HTTP.
                self.reply(400, {'error': type(exc).__name__ + ': embedding failed'})
    return Handler


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    for name in ['model', 'root', 'device', 'gpu-device', 'modalities', 'api-key']:
        parser.add_argument('--' + name, required=True)
    parser.add_argument('--port', type=int, required=True)
    parser.add_argument('--threads', type=int, default=8)
    args = parser.parse_args()
    import torch
    from sentence_transformers import SentenceTransformer
    torch.set_num_threads(args.threads)
    if args.device == 'cpu':
        device, dtype = 'cpu', torch.float32
    else:
        if not torch.cuda.is_available():
            raise RuntimeError('GPU backend unavailable in this portable PyTorch runtime')
        device = args.gpu_device or 'cuda:0'  # PyTorch uses cuda also for ROCm/HIP.
        if not device.startswith('cuda:'):
            raise ValueError('PyTorch GPU device must be cuda:N (also on AMD ROCm)')
        torch.cuda.set_device(device)
        dtype = torch.bfloat16 if torch.cuda.is_bf16_supported() else torch.float32
    model = SentenceTransformer(args.model, device=device, local_files_only=True,
                                config_kwargs=selective_config(args.modalities),
                                model_kwargs={'torch_dtype': dtype})
    configure_model(model)
    server = ThreadingHTTPServer(('127.0.0.1', args.port), make_handler(
        model, Path(args.root), set(args.modalities.split(',')), args.api_key, threading.Lock()))
    server.daemon_threads = True
    server.serve_forever()


if __name__ == '__main__':
    main()
