# Canal

_was channelOS_

```
# Some notes on getting started
brew install tinygo
pip3 install pyserial      # for make monitor / make run
git clone …
cd Canal/canal
make flash                 # auto-detects /dev/cu.usbmodem*
make run                   # flash + open monitor in one step
```

## 📚 Educational Materials

A planned series of articles covering Canal and its companion language
[picoceci](https://github.com/kristofer/picoceci):

| # | Article |
|---|---------|
| 1 | [The ESP32-S3 System: Hardware Meets Software](docs/articles/01-esp32s3-hardware-meets-software.md) |
| 2 | [picoceci: A Language Built for Tiny Machines](docs/articles/02-picoceci-a-language-for-tiny-machines.md) |
| 3 | [Canal and FreeRTOS: Running Go on Bare Metal](docs/articles/03-canal-and-freertos-go-on-bare-metal.md) |
| 4 | [picoceci on Canal: Programming the Microkernel](docs/articles/04-picoceci-on-canal-programming-the-microkernel.md) |
| 5 | [Build a Programming Learning Environment on Canal](docs/articles/05-build-a-programming-learning-environment.md) |

See **[docs/EDUCATIONAL_PLAN.md](docs/EDUCATIONAL_PLAN.md)** for the full plan,
including learning objectives, key topics, target audience, and production notes for
each article.

