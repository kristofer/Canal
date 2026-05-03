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

**Canal** is a microkernel operating system for the ESP32-S3, written in Go and built on top of FreeRTOS. It is designed to be a platform for learning about
operating system concepts, embedded programming, and the Go programming language. Canal is a work in progress, and is currently in the early stages of development. The project is open source and welcomes contributions from the community.

**Picoceci** is a companion programming language for Canal, designed to be simple and easy to learn. It is a small, statically typed language that compiles to Go, and is intended to be used for programming the Canal microkernel and applications running on it. See the [picoceci repository](https://github.com/kristofer/picoceci) for more information.

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
