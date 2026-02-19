"""
FastAPI application entry point for the AI sidecar service.
"""

import asyncio
import signal
import sys
from contextlib import asynccontextmanager
from typing import Optional

import uvicorn
from fastapi import FastAPI, HTTPException
from fastapi.middleware.cors import CORSMiddleware

from ai_sidecar.models import (
    APIResponse,
    AnalyzeRequest,
    AnalyzeResult,
    DeduplicateRequest,
    IdiomatizeRequest,
    PatternRequest,
    ApplyPlanRequest,
    EmbedRequest,
    RefactorPlan,
    RefactorResult,
)
from ai_sidecar.agents import (
    AnalyzerAgent,
    DeduplicatorAgent,
    IdiomatizerAgent,
    PatternAgent,
    ValidatorAgent,
)
from ai_sidecar.embeddings import EmbeddingService

embedding_service: Optional[EmbeddingService] = None
analyzer_agent: Optional[AnalyzerAgent] = None
deduplicator_agent: Optional[DeduplicatorAgent] = None
idiomatizer_agent: Optional[IdiomatizerAgent] = None
pattern_agent: Optional[PatternAgent] = None
validator_agent: Optional[ValidatorAgent] = None

shutdown_event = asyncio.Event()


@asynccontextmanager
async def lifespan(app: FastAPI):
    global embedding_service, analyzer_agent, deduplicator_agent
    global idiomatizer_agent, pattern_agent, validator_agent

    embedding_service = EmbeddingService()
    await embedding_service.initialize()

    analyzer_agent = AnalyzerAgent()
    deduplicator_agent = DeduplicatorAgent(embedding_service)
    idiomatizer_agent = IdiomatizerAgent()
    pattern_agent = PatternAgent()
    validator_agent = ValidatorAgent()

    yield

    await embedding_service.shutdown()


app = FastAPI(
    title="DeHydrator AI Sidecar",
    description="AI-powered code analysis and refactoring service",
    version="0.1.0",
    lifespan=lifespan,
)

app.add_middleware(
    CORSMiddleware,
    allow_origins=["*"],
    allow_credentials=True,
    allow_methods=["*"],
    allow_headers=["*"],
)


@app.get("/health")
async def health_check():
    return {"status": "healthy", "service": "ai_sidecar"}


@app.post("/shutdown")
async def shutdown():
    shutdown_event.set()
    asyncio.get_event_loop().call_later(0.5, lambda: sys.exit(0))
    return {"status": "shutting_down"}


@app.post("/analyze", response_model=APIResponse)
async def analyze_endpoint(request: AnalyzeRequest):
    try:
        result = await analyzer_agent.analyze(request)
        return APIResponse(status="success", data=result.model_dump())
    except Exception as e:
        raise HTTPException(status_code=500, detail=str(e))


@app.post("/deduplicate", response_model=APIResponse)
async def deduplicate_endpoint(request: DeduplicateRequest):
    try:
        plan = await deduplicator_agent.find_duplicates(request)
        return APIResponse(status="success", data=plan.model_dump())
    except Exception as e:
        raise HTTPException(status_code=500, detail=str(e))


@app.post("/idiomatize", response_model=APIResponse)
async def idiomatize_endpoint(request: IdiomatizeRequest):
    try:
        plan = await idiomatizer_agent.idiomatize(request)
        return APIResponse(status="success", data=plan.model_dump())
    except Exception as e:
        raise HTTPException(status_code=500, detail=str(e))


@app.post("/pattern", response_model=APIResponse)
async def pattern_endpoint(request: PatternRequest):
    try:
        plan = await pattern_agent.apply_pattern(request)
        return APIResponse(status="success", data=plan.model_dump())
    except Exception as e:
        raise HTTPException(status_code=500, detail=str(e))


@app.post("/apply", response_model=APIResponse)
async def apply_plan_endpoint(request: ApplyPlanRequest):
    try:
        result = await validator_agent.apply_plan(request.session_id)
        return APIResponse(status="success", data=result.model_dump())
    except Exception as e:
        raise HTTPException(status_code=500, detail=str(e))


@app.post("/embed", response_model=APIResponse)
async def embed_endpoint(request: EmbedRequest):
    try:
        embeddings = await embedding_service.embed_files(request.files)
        return APIResponse(status="success", data=embeddings)
    except Exception as e:
        raise HTTPException(status_code=500, detail=str(e))


def run_server(port: int = 9876):
    uvicorn.run(
        "ai_sidecar.main:app",
        host="127.0.0.1",
        port=port,
        log_level="warning",
    )


if __name__ == "__main__":
    import argparse

    parser = argparse.ArgumentParser(description="DeHydrator AI Sidecar")
    parser.add_argument("--port", type=int, default=9876, help="Port to listen on")
    args = parser.parse_args()

    run_server(args.port)
