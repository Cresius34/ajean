"""Protocol/security tests, no model download required."""
import importlib.util
import json
from pathlib import Path
import tempfile
import threading
import unittest
from urllib.request import Request,urlopen
from urllib.error import HTTPError
from http.server import ThreadingHTTPServer

path=Path(__file__).resolve().parents[2]/'internal/ajean/embedding_server.py'
spec=importlib.util.spec_from_file_location('embedding_server',path)
worker=importlib.util.module_from_spec(spec);spec.loader.exec_module(worker)

class WorkerTest(unittest.TestCase):
    def test_selective_loading(self):
        self.assertEqual(worker.selective_config('text'),{'vision_config':None,'audio_config':None})
        self.assertEqual(worker.selective_config('text,image'),{'audio_config':None})
        self.assertEqual(worker.selective_config('text,video'),{'audio_config':None})
        self.assertEqual(worker.selective_config('text,audio'),{'vision_config':None})
        self.assertEqual(worker.selective_config('text,image,audio,video'),{})
        with self.assertRaises(ValueError):worker.selective_config('image')

    def test_auth_and_media_scope(self):
        class Rows:
            def tolist(self):return [[1.]+[0.]*767]
        class Model:
            def encode(self,inputs,**kwargs):
                self.inputs=inputs
                return Rows()
        model=Model()
        with tempfile.TemporaryDirectory() as tmp:
            root=Path(tmp);image=root/'image.png';image.write_bytes(b'fixture')
            server=ThreadingHTTPServer(('127.0.0.1',0),worker.make_handler(model,root,{'text','image'},'secret',threading.Lock()))
            thread=threading.Thread(target=server.serve_forever,daemon=True);thread.start()
            try:
                base='http://127.0.0.1:'+str(server.server_port)
                with self.assertRaises(HTTPError) as error:urlopen(base+'/health')
                self.assertEqual(error.exception.code,401)
                data={'input':[{'text':'title: note | text: <|image|>','image':[str(image)]}]}
                req=Request(base+'/v1/embeddings',data=json.dumps(data).encode(),headers={'Authorization':'Bearer secret'})
                with urlopen(req) as response:self.assertEqual(len(json.load(response)['data'][0]['embedding']),768)
                self.assertEqual(model.inputs[0]['image'],[str(image)])
                data['input'][0]['image']=['/etc/passwd']
                req=Request(base+'/v1/embeddings',data=json.dumps(data).encode(),headers={'Authorization':'Bearer secret'})
                with self.assertRaises(HTTPError) as error:urlopen(req)
                self.assertEqual(error.exception.code,400)
                with self.assertRaises(ValueError):worker.prepare_input({'audio':str(image)},root,{'text','image'})
            finally:server.shutdown();server.server_close();thread.join()

if __name__=='__main__':unittest.main()
