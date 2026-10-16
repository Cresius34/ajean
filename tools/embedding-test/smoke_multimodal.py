#!/usr/bin/env python3
"""Opt-in proof with real EmbeddingGemma 2 weights (CPU, no GPU claim).
Run using the interpreter with embedding-requirements + CPU torch/torchvision.
"""
import argparse
import importlib.util
import json
from pathlib import Path
import tempfile
import torch
from sentence_transformers import SentenceTransformer
from PIL import Image


def main():
    ap=argparse.ArgumentParser(description=__doc__)
    ap.add_argument('--model',type=Path,required=True)
    ap.add_argument('--report',type=Path,required=True)
    args=ap.parse_args()
    torch.set_num_threads(4)
    script=Path(__file__).resolve().parents[2]/'internal/ajean/embedding_server.py'
    spec=importlib.util.spec_from_file_location('embedding_server',script)
    worker=importlib.util.module_from_spec(spec);spec.loader.exec_module(worker)
    report={'environment':'Linux x86_64 CPU / float32','model_revision':'914f7f89142e33e77833254d9c9b90c3cef7303b'}
    with tempfile.TemporaryDirectory(prefix='ajean-modal-proof-') as tmp:
        root=Path(tmp);image=root/'red.png';Image.new('RGB',(224,224),(220,20,20)).save(image)
        import numpy as np,soundfile as sf,av
        audio=root/'tone.wav';sf.write(audio,np.sin(np.arange(16000)*2*np.pi*440/16000).astype('float32')*.1,16000)
        video=root/'red.mp4'
        container=av.open(str(video),'w');stream=container.add_stream('mpeg4',rate=1);stream.width=224;stream.height=224;stream.pix_fmt='yuv420p'
        for _ in range(2):
            frame=av.VideoFrame.from_image(Image.open(image))
            for packet in stream.encode(frame):container.mux(packet)
        for packet in stream.encode():container.mux(packet)
        container.close()
        for active,item in [('text','task: search result | query: couleur rouge'),('text,image',{'text':'<|image|>','image':[str(image)]}),('text,audio',{'text':'<|audio|>','audio':str(audio)}),('text,image,audio,video',{'text':'<|video|>','video':str(video)})]:
            model=SentenceTransformer(str(args.model),device='cpu',local_files_only=True,
                config_kwargs=worker.selective_config(active),model_kwargs={'torch_dtype':torch.float32})
            worker.configure_model(model)
            item=worker.materialize_video([item])[0]
            v=model.encode([item],normalize_embeddings=True,show_progress_bar=False)
            assert v.shape==(1,768) and np.isfinite(v).all()
            assert abs(float(np.linalg.norm(v[0]))-1)<.001
            params=sum(p.numel() for p in model.parameters())
            report[active]={'result':'PASS','loaded_parameters':params,'dimensions':768}
            del model
            import gc;gc.collect()
    args.report.write_text(json.dumps(report,indent=2)+'\n')
    print(json.dumps(report))


if __name__=='__main__':main()
