import os
import io
import logging
from typing import List, Optional
import numpy as np
import cv2
from fastapi import FastAPI, UploadFile, File, HTTPException, status
from fastapi.middleware.cors import CORSMiddleware
from pydantic import BaseModel
import insightface
from insightface.app import FaceAnalysis

logging.basicConfig(level=logging.INFO, format="%(asctime)s [%(levelname)s] %(message)s")
logger = logging.getLogger("ai-service")

app = FastAPI(title="AIFS AI Service", version="1.0.0")

app.add_middleware(
    CORSMiddleware,
    allow_origins=["*"],
    allow_credentials=True,
    allow_methods=["*"],
    allow_headers=["*"],
)

MODEL_NAME = os.getenv("INSIGHTFACE_MODEL", "buffalo_sc")
MODELS_ROOT = os.getenv("INSIGHTFACE_ROOT", "/models")

face_app: Optional[FaceAnalysis] = None
is_ready = False

@app.on_event("startup")
def startup_event():
    global face_app, is_ready
    logger.info(f"Initializing InsightFace with model: {MODEL_NAME}, root: {MODELS_ROOT}")
    try:
        # providers=['CPUExecutionProvider']
        face_app = FaceAnalysis(name=MODEL_NAME, root=MODELS_ROOT, providers=['CPUExecutionProvider'])
        face_app.prepare(ctx_id=0, det_size=(640, 640))
        is_ready = True
        logger.info(f"InsightFace model '{MODEL_NAME}' successfully loaded and ready.")
    except Exception as e:
        logger.error(f"Failed to load InsightFace model: {e}", exc_info=True)
        # We don't crash container immediately so logs are inspectable
        is_ready = False

class BoundingBox(BaseModel):
    x: float
    y: float
    width: float
    height: float

class DetectedFace(BaseModel):
    bbox: BoundingBox
    confidence: float
    embedding: List[float]

class DetectionResponse(BaseModel):
    count: int
    faces: List[DetectedFace]
    model_name: str
    model_version: str

class SelfieFaceResponse(BaseModel):
    face: DetectedFace
    model_name: str
    model_version: str

def read_image_from_bytes(contents: bytes) -> np.ndarray:
    nparr = np.frombuffer(contents, np.uint8)
    img = cv2.imdecode(nparr, cv2.IMREAD_COLOR)
    if img is None:
        raise HTTPException(
            status_code=status.HTTP_400_BAD_REQUEST,
            detail="Could not decode image file. Unsupported format or corrupted data."
        )
    return img

@app.get("/health")
def health_check():
    return {
        "status": "healthy" if is_ready else "initializing",
        "ready": is_ready,
        "model": MODEL_NAME,
        "root": MODELS_ROOT
    }

@app.post("/detect-and-embed", response_model=DetectionResponse)
async def detect_and_embed(file: UploadFile = File(...)):
    if not is_ready or face_app is None:
        raise HTTPException(status_code=status.HTTP_503_SERVICE_UNAVAILABLE, detail="AI model is not ready")

    contents = await file.read()
    img = read_image_from_bytes(contents)

    raw_faces = face_app.get(img)
    results: List[DetectedFace] = []

    for f in raw_faces:
        # bbox is [x1, y1, x2, y2]
        x1, y1, x2, y2 = f.bbox.tolist()
        w = max(0.0, float(x2 - x1))
        h = max(0.0, float(y2 - y1))
        conf = float(f.det_score) if hasattr(f, 'det_score') else 1.0

        # normalized embedding
        normed_emb = f.normed_embedding.tolist() if hasattr(f, 'normed_embedding') else f.embedding.tolist()

        results.append(
            DetectedFace(
                bbox=BoundingBox(x=float(x1), y=float(y1), width=w, height=h),
                confidence=conf,
                embedding=normed_emb
            )
        )

    logger.info(f"Processed image '{file.filename}': detected {len(results)} faces")
    return DetectionResponse(
        count=len(results),
        faces=results,
        model_name="insightface",
        model_version=MODEL_NAME
    )

@app.post("/embed-selfie", response_model=SelfieFaceResponse)
async def embed_selfie(file: UploadFile = File(...)):
    if not is_ready or face_app is None:
        raise HTTPException(status_code=status.HTTP_503_SERVICE_UNAVAILABLE, detail="AI model is not ready")

    contents = await file.read()
    img = read_image_from_bytes(contents)

    raw_faces = face_app.get(img)
    count = len(raw_faces)

    if count == 0:
        raise HTTPException(
            status_code=status.HTTP_400_BAD_REQUEST,
            detail={"error": "NO_FACE_DETECTED", "message": "Không tìm thấy khuôn mặt nào trong ảnh selfie. Vui lòng chụp rõ mặt."}
        )

    if count > 1:
        raise HTTPException(
            status_code=status.HTTP_400_BAD_REQUEST,
            detail={
                "error": "MULTIPLE_FACES_DETECTED",
                "faces_count": count,
                "message": f"Phát hiện {count} khuôn mặt. Vui lòng tải ảnh chỉ có một mình bạn để tìm kiếm chính xác."
            }
        )

    f = raw_faces[0]
    x1, y1, x2, y2 = f.bbox.tolist()
    w = max(0.0, float(x2 - x1))
    h = max(0.0, float(y2 - y1))
    conf = float(f.det_score) if hasattr(f, 'det_score') else 1.0
    normed_emb = f.normed_embedding.tolist() if hasattr(f, 'normed_embedding') else f.embedding.tolist()

    return SelfieFaceResponse(
        face=DetectedFace(
            bbox=BoundingBox(x=float(x1), y=float(y1), width=w, height=h),
            confidence=conf,
            embedding=normed_emb
        ),
        model_name="insightface",
        model_version=MODEL_NAME
    )
