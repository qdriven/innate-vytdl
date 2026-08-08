#!/usr/bin/env python3
"""
YouTube URL Batch Downloader

This script reads a text file containing YouTube URLs (one per line)
and downloads them using vYtDL.

Usage:
    python batch_download.py <url_file.txt> [output_dir] [options]

Examples:
    python batch_download.py youtube_urls.txt ./downloads
    python batch_download.py youtube_urls.txt ./downloads --quality 1080
    python batch_download.py youtube_urls.txt ./downloads --playlist
"""

import argparse
import os
import subprocess
import sys
from pathlib import Path
from typing import List, Optional


def read_urls(file_path: str) -> List[str]:
    """Read URLs from a text file, one URL per line."""
    urls = []
    with open(file_path, 'r', encoding='utf-8') as f:
        for line in f:
            url = line.strip()
            if url and not url.startswith('#'):
                urls.append(url)
    return urls


def find_vytdl() -> Optional[str]:
    """Find the vYtDL binary."""
    candidates = [
        './vYtDL',
        '../vYtDL/vYtDL',
        '../../vYtDL/vYtDL',
        'vYtDL',
    ]
    
    for candidate in candidates:
        if os.path.isfile(candidate):
            return candidate
        if os.path.isfile(candidate + '.exe'):
            return candidate + '.exe'
    
    try:
        result = subprocess.run(['which', 'vYtDL'], capture_output=True, text=True)
        if result.returncode == 0:
            return result.stdout.strip()
    except FileNotFoundError:
        pass
    
    try:
        result = subprocess.run(['where', 'vYtDL'], capture_output=True, text=True, shell=True)
        if result.returncode == 0:
            return result.stdout.strip().split('\n')[0]
    except FileNotFoundError:
        pass
    
    return None


def download_videos(
    urls: List[str],
    output_dir: str,
    quality: str = '',
    format_type: str = 'mp4',
    is_playlist: bool = False,
    no_tui: bool = True,
    extra_args: List[str] = None
) -> tuple:
    """Download videos using vYtDL."""
    
    vytdl_bin = find_vytdl()
    if not vytdl_bin:
        print("Error: vYtDL binary not found.")
        print("Please ensure vYtDL is built and accessible.")
        print("Build it with: cd vYtDL && go build -o vYtDL .")
        return 0, len(urls)
    
    output_dir = os.path.abspath(output_dir)
    os.makedirs(output_dir, exist_ok=True)
    
    cmd = [vytdl_bin, 'download']
    
    if quality:
        cmd.extend(['--quality', quality])
    
    cmd.extend(['--format', format_type])
    cmd.extend(['--output', output_dir])
    
    if is_playlist:
        cmd.append('--playlist')
    
    if extra_args:
        cmd.extend(extra_args)
    
    success_count = 0
    fail_count = 0
    
    total = len(urls)
    for i, url in enumerate(urls, 1):
        print(f"\n[{i}/{total}] Downloading: {url}")
        
        try:
            result = subprocess.run(
                cmd + [url],
                capture_output=False,
                text=True
            )
            
            if result.returncode == 0:
                success_count += 1
                print(f"[{i}/{total}] ✓ Success")
            else:
                fail_count += 1
                print(f"[{i}/{total}] ✗ Failed (exit code: {result.returncode})")
                
        except Exception as e:
            fail_count += 1
            print(f"[{i}/{total}] ✗ Error: {e}")
    
    return success_count, fail_count


def main():
    parser = argparse.ArgumentParser(
        description='Batch download YouTube videos using vYtDL',
        formatter_class=argparse.RawDescriptionHelpFormatter,
        epilog='''
Examples:
  %(prog)s urls.txt ./downloads
  %(prog)s urls.txt ./downloads --quality 1080
  %(prog)s urls.txt ./downloads --quality 720 --format webm
  %(prog)s playlist_urls.txt ./downloads --playlist

The URL file should contain one YouTube URL per line.
Lines starting with # are treated as comments.
        '''
    )
    
    parser.add_argument(
        'url_file',
        help='Path to text file containing YouTube URLs (one per line)'
    )
    
    parser.add_argument(
        'output_dir',
        nargs='?',
        default='./downloads',
        help='Output directory for downloaded videos (default: ./downloads)'
    )
    
    parser.add_argument(
        '-q', '--quality',
        default='',
        help='Video quality: 720, 1080, 2160, etc. (default: best)'
    )
    
    parser.add_argument(
        '-f', '--format',
        default='mp4',
        help='Output format: mp4, webm, mkv (default: mp4)'
    )
    
    parser.add_argument(
        '-p', '--playlist',
        action='store_true',
        help='Treat URLs as playlists'
    )
    
    parser.add_argument(
        '--log-format',
        choices=['json', 'csv'],
        default='json',
        help='Download record format (default: json)'
    )
    
    parser.add_argument(
        '--extra',
        nargs=argparse.REMAINDER,
        help='Extra arguments to pass to vYtDL'
    )
    
    args = parser.parse_args()
    
    if not os.path.isfile(args.url_file):
        print(f"Error: URL file not found: {args.url_file}")
        sys.exit(1)
    
    urls = read_urls(args.url_file)
    
    if not urls:
        print("Error: No URLs found in the file")
        sys.exit(1)
    
    print(f"Found {len(urls)} URL(s) to download")
    print(f"Output directory: {os.path.abspath(args.output_dir)}")
    print(f"Quality: {args.quality or 'best'}")
    print(f"Format: {args.format}")
    print(f"Playlist mode: {'Yes' if args.playlist else 'No'}")
    
    extra_args = []
    if args.extra:
        extra_args = args.extra
    
    extra_args.extend(['--log-format', args.log_format])
    
    success, fail = download_videos(
        urls=urls,
        output_dir=args.output_dir,
        quality=args.quality,
        format_type=args.format,
        is_playlist=args.playlist,
        extra_args=extra_args
    )
    
    print("\n" + "="*50)
    print("Download Summary")
    print("="*50)
    print(f"Total URLs:  {len(urls)}")
    print(f"Succeeded:   {success}")
    print(f"Failed:      {fail}")
    
    if fail > 0:
        sys.exit(1)


if __name__ == '__main__':
    main()
