"""
Embedding service for semantic code analysis.
"""

import hashlib
from typing import List, Dict, Optional
import chromadb
from chromadb.config import Settings

from ai_sidecar.models import FileInfo, CodeBlock, Language


class EmbeddingService:
    def __init__(self):
        self.client: Optional[chromadb.Client] = None
        self.collection = None
        self._initialized = False

    async def initialize(self):
        if self._initialized:
            return

        self.client = chromadb.Client(Settings(
            chroma_db_impl="duckdb+parquet",
            persist_directory=None,
        ))

        self.collection = self.client.get_or_create_collection(
            name="code_embeddings",
            metadata={"hnsw:space": "cosine"}
        )

        self._initialized = True

    async def shutdown(self):
        if self.client:
            self.client = None
            self.collection = None
            self._initialized = False

    def _mock_embedding(self, text: str) -> List[float]:
        h = hashlib.sha256(text.encode()).hexdigest()
        embedding = []
        for i in range(0, 64, 4):
            val = int(h[i:i+4], 16) / 65535.0
            embedding.append(val)
        return embedding[:384]

    async def embed_text(self, text: str) -> List[float]:
        return self._mock_embedding(text)

    async def embed_files(self, files: List[FileInfo]) -> Dict[str, List[float]]:
        result = {}
        for file in files:
            result[file.path] = await self.embed_text(file.content)
        return result

    async def embed_blocks(self, blocks: List[CodeBlock]) -> List[CodeBlock]:
        for block in blocks:
            block.embedding = await self.embed_text(block.content)
        return blocks

    async def store_embeddings(self, blocks: List[CodeBlock]):
        if not self._initialized or not self.collection:
            return

        ids = [block.id for block in blocks]
        embeddings = [block.embedding for block in blocks if block.embedding]
        metadatas = [
            {
                "file": block.file,
                "start_line": block.start_line,
                "end_line": block.end_line,
                "symbol_type": block.symbol_type,
                "symbol_name": block.symbol_name,
                "language": block.language.value,
            }
            for block in blocks
        ]
        documents = [block.content for block in blocks]

        if embeddings:
            self.collection.add(
                ids=ids[:len(embeddings)],
                embeddings=embeddings,
                metadatas=metadatas[:len(embeddings)],
                documents=documents[:len(embeddings)],
            )

    async def find_similar(
        self,
        embedding: List[float],
        n_results: int = 5,
        where: Optional[Dict] = None,
    ) -> List[Dict]:
        if not self._initialized or not self.collection:
            return []

        results = self.collection.query(
            query_embeddings=[embedding],
            n_results=n_results,
            where=where,
        )

        similar = []
        for i, id in enumerate(results["ids"][0]):
            similar.append({
                "id": id,
                "distance": results["distances"][0][i] if results.get("distances") else 0,
                "metadata": results["metadatas"][0][i] if results.get("metadatas") else {},
                "content": results["documents"][0][i] if results.get("documents") else "",
            })

        return similar

    async def find_duplicates(
        self,
        blocks: List[CodeBlock],
        threshold: float = 0.85,
    ) -> List[List[CodeBlock]]:
        if not self._initialized:
            await self.initialize()

        blocks_with_embeddings = await self.embed_blocks(blocks)
        await self.store_embeddings(blocks_with_embeddings)

        groups = []
        processed = set()

        for block in blocks_with_embeddings:
            if block.id in processed or not block.embedding:
                continue

            similar = await self.find_similar(
                block.embedding,
                n_results=10,
            )

            group = [block]
            processed.add(block.id)

            for sim in similar:
                if sim["id"] == block.id:
                    continue

                similarity = 1 - sim["distance"]
                if similarity >= threshold:
                    for b in blocks_with_embeddings:
                        if b.id == sim["id"] and b.id not in processed:
                            group.append(b)
                            processed.add(b.id)
                            break

            if len(group) > 1:
                groups.append(group)

        return groups

    async def clear(self):
        if self._initialized and self.client:
            self.client.delete_collection("code_embeddings")
            self.collection = self.client.get_or_create_collection(
                name="code_embeddings",
                metadata={"hnsw:space": "cosine"}
            )
